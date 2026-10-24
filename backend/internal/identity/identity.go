package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnauthorized     = errors.New("authentication required")
	ErrForbidden        = errors.New("access denied by default policy")
	ErrNotFound         = errors.New("resource not found")
	ErrInvalidChallenge = errors.New("invalid or expired authentication challenge")
	ErrInvalidMFA       = errors.New("invalid MFA verification code")
	ErrMFARequired      = errors.New("MFA verification required")
	ErrInvalidInput     = errors.New("invalid input")
)

type Preferences struct {
	AccountID             string    `json:"account_id"`
	SafeSearch            string    `json:"safe_search"`
	Theme                 string    `json:"theme"`
	Language              string    `json:"language"`
	PersonalizedSearch    bool      `json:"personalized_search"`
	RecentSearchesOptIn   bool      `json:"recent_searches_opt_in"`
	SearchCorrectionOptIn bool      `json:"search_correction_opt_in"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type Account struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	Locale      string    `json:"locale"`
	Roles       []string  `json:"roles"`
	MFAEnrolled bool      `json:"mfa_enrolled"`
	CreatedAt   time.Time `json:"created_at"`
}

type Session struct {
	ID          string     `json:"id"`
	AccountID   string     `json:"account_id"`
	TokenHash   string     `json:"-"`
	MFAVerified bool       `json:"mfa_verified"`
	ExpiresAt   time.Time  `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type OrganizationMember struct {
	OrganizationID string `json:"organization_id"`
	AccountID      string `json:"account_id"`
	Role           string `json:"role"`
}

type Site struct {
	ID                string    `json:"id"`
	OrganizationID    string    `json:"organization_id"`
	OwnerID           string    `json:"owner_id"`
	Origin            string    `json:"origin"`
	VerificationState string    `json:"verification_state"`
	AddedAt           time.Time `json:"added_at"`
}

type Challenge struct {
	ID            string    `json:"challenge_id"`
	AccountID     string    `json:"account_id,omitempty"`
	Email         string    `json:"email,omitempty"`
	ChallengeType string    `json:"challenge_type"`
	ChallengeText string    `json:"challenge"`
	RPID          string    `json:"rp_id"`
	ExpiresAt     time.Time `json:"expires_at"`
	Consumed      bool      `json:"-"`
}

type Store struct {
	mu             sync.RWMutex
	accounts       map[string]*Account
	byEmail        map[string]string
	sessions       map[string]*Session // keyed by hex sha256(token)
	sessionsByID   map[string]*Session
	prefs          map[string]*Preferences
	mfaSecrets     map[string]string   // accountID -> base32 secret
	mfaPending     map[string]string   // accountID -> pending base32 secret
	recoveryCodes  map[string][]string // accountID -> list of sha256(code)
	recentSearches map[string][]string // accountID -> recent queries (only when RecentSearchesOptIn=true)
	challenges     map[string]*Challenge
	orgMembers     map[string]map[string]string // orgID -> accountID -> role
	sites          map[string]*Site
	nowFunc        func() time.Time
}

func NewStore() *Store {
	s := &Store{
		accounts:       make(map[string]*Account),
		byEmail:        make(map[string]string),
		sessions:       make(map[string]*Session),
		sessionsByID:   make(map[string]*Session),
		prefs:          make(map[string]*Preferences),
		mfaSecrets:     make(map[string]string),
		mfaPending:     make(map[string]string),
		recoveryCodes:  make(map[string][]string),
		recentSearches: make(map[string][]string),
		challenges:     make(map[string]*Challenge),
		orgMembers:     make(map[string]map[string]string),
		sites:          make(map[string]*Site),
		nowFunc:        time.Now,
	}
	return s
}

func randomHex(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// CreateChallenge issues a WebAuthn Passkey or OIDC nonce challenge with a 5-minute TTL.
func (s *Store) CreateChallenge(challengeType, email, rpID string) (*Challenge, error) {
	switch challengeType {
	case "passkey_login", "passkey_register", "oidc_nonce":
	default:
		return nil, ErrInvalidInput
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, ErrInvalidInput
	}
	if rpID == "" {
		rpID = "localhost"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := &Challenge{
		ID:            "ch-" + randomHex(8),
		Email:         email,
		ChallengeType: challengeType,
		ChallengeText: randomHex(16),
		RPID:          rpID,
		ExpiresAt:     s.nowFunc().UTC().Add(5 * time.Minute),
	}
	s.challenges[ch.ID] = ch
	return ch, nil
}

// CompleteChallenge verifies a challenge and authenticates or registers the account, issuing a session token and one-time recovery codes on registration.
func (s *Store) CompleteChallenge(challengeID, challengeResponse, displayName string) (rawToken string, session *Session, account *Account, recoveryCodes []string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.challenges[challengeID]
	if !ok || ch.Consumed || s.nowFunc().UTC().After(ch.ExpiresAt) {
		return "", nil, nil, nil, ErrInvalidChallenge
	}
	expected := "signed:" + ch.ChallengeText
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(challengeResponse)), []byte(expected)) != 1 {
		return "", nil, nil, nil, ErrInvalidChallenge
	}
	ch.Consumed = true

	accID, exists := s.byEmail[ch.Email]
	if !exists {
		accID = "acc-" + randomHex(8)
		if strings.TrimSpace(displayName) == "" {
			displayName = strings.Split(ch.Email, "@")[0]
		}
		acc := &Account{
			ID:          accID,
			Email:       ch.Email,
			DisplayName: strings.TrimSpace(displayName),
			Status:      "active",
			Locale:      "ko-KR",
			Roles:       []string{"user"},
			CreatedAt:   s.nowFunc().UTC(),
		}
		s.accounts[accID] = acc
		s.byEmail[ch.Email] = accID
		s.prefs[accID] = &Preferences{
			AccountID:             accID,
			SafeSearch:            "moderate",
			Theme:                 "system",
			Language:              "ko",
			PersonalizedSearch:    false,
			RecentSearchesOptIn:   false,
			SearchCorrectionOptIn: true,
			UpdatedAt:             s.nowFunc().UTC(),
		}
		// Generate 4 one-time recovery codes
		codes := make([]string, 4)
		hashes := make([]string, 4)
		for i := 0; i < 4; i++ {
			c := fmt.Sprintf("OP-%s-%s", strings.ToUpper(randomHex(2)), strings.ToUpper(randomHex(2)))
			codes[i] = c
			hashes[i] = hashToken(c)
		}
		s.recoveryCodes[accID] = hashes
		recoveryCodes = codes
	}

	acc := s.accounts[accID]
	rawToken = "ops_" + randomHex(24)
	sess := &Session{
		ID:          "sess-" + randomHex(8),
		AccountID:   acc.ID,
		TokenHash:   hashToken(rawToken),
		MFAVerified: !acc.MFAEnrolled,
		ExpiresAt:   s.nowFunc().UTC().Add(12 * time.Hour),
		CreatedAt:   s.nowFunc().UTC(),
	}
	s.sessions[sess.TokenHash] = sess
	s.sessionsByID[sess.ID] = sess
	accCopy := *acc
	return rawToken, sess, &accCopy, recoveryCodes, nil
}

// VerifySession validates a bearer/cookie token and returns the active session and account.
func (s *Store) VerifySession(rawToken string) (*Session, *Account, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, nil, ErrUnauthorized
	}
	h := hashToken(strings.TrimSpace(rawToken))
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[h]
	if !ok || sess.RevokedAt != nil || s.nowFunc().UTC().After(sess.ExpiresAt) {
		return nil, nil, ErrUnauthorized
	}
	acc, ok := s.accounts[sess.AccountID]
	if !ok || acc.Status != "active" {
		return nil, nil, ErrUnauthorized
	}
	sessCopy := *sess
	accCopy := *acc
	return &sessCopy, &accCopy, nil
}

// RevokeSession immediately revokes the current session token.
func (s *Store) RevokeSession(rawToken string) error {
	h := hashToken(strings.TrimSpace(rawToken))
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[h]
	if !ok || sess.RevokedAt != nil {
		return ErrUnauthorized
	}
	now := s.nowFunc().UTC()
	sess.RevokedAt = &now
	return nil
}

// RevokeSessionByID allows an account to revoke its own session by session ID, enforcing BOLA checks.
func (s *Store) RevokeSessionByID(actorAccountID, targetSessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessionsByID[targetSessionID]
	if !ok {
		return ErrNotFound
	}
	if sess.AccountID != actorAccountID {
		return ErrForbidden
	}
	now := s.nowFunc().UTC()
	sess.RevokedAt = &now
	return nil
}

// EnrollMFA generates a base32 TOTP secret for an authenticated account.
func (s *Store) EnrollMFA(accountID string) (secret string, otpauthURI string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accounts[accountID]
	if !ok {
		return "", "", ErrUnauthorized
	}
	raw := make([]byte, 20)
	_, _ = rand.Read(raw)
	secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	s.mfaPending[accountID] = secret
	uri := fmt.Sprintf("otpauth://totp/OpenPortal:%s?secret=%s&issuer=OpenPortal&algorithm=SHA1&digits=6&period=30", acc.Email, secret)
	return secret, uri, nil
}

// GenerateTOTP computes a 6-digit RFC 6238 TOTP code for the given base32 secret and time.
func GenerateTOTP(base32Secret string, t time.Time) (string, error) {
	clean := strings.ToUpper(strings.TrimSpace(base32Secret))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(clean)
	if err != nil {
		return "", err
	}
	counter := uint64(t.UTC().Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binCode := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)
	otp := binCode % 1000000
	return fmt.Sprintf("%06d", otp), nil
}

// VerifyAndActivateMFA verifies a TOTP code against the pending or active secret and marks the session MFA-verified.
func (s *Store) VerifyAndActivateMFA(rawToken, code string) error {
	h := hashToken(strings.TrimSpace(rawToken))
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[h]
	if !ok || sess.RevokedAt != nil || s.nowFunc().UTC().After(sess.ExpiresAt) {
		return ErrUnauthorized
	}
	acc, ok := s.accounts[sess.AccountID]
	if !ok {
		return ErrUnauthorized
	}
	secret := s.mfaSecrets[acc.ID]
	isPending := false
	if pending, ok := s.mfaPending[acc.ID]; ok && pending != "" {
		secret = pending
		isPending = true
	}
	if secret == "" {
		return ErrInvalidMFA
	}
	now := s.nowFunc().UTC()
	valid := false
	for _, offset := range []time.Duration{-30 * time.Second, 0, 30 * time.Second} {
		expected, err := GenerateTOTP(secret, now.Add(offset))
		if err == nil && subtle.ConstantTimeCompare([]byte(strings.TrimSpace(code)), []byte(expected)) == 1 {
			valid = true
			break
		}
	}
	if !valid {
		return ErrInvalidMFA
	}
	if isPending {
		s.mfaSecrets[acc.ID] = secret
		delete(s.mfaPending, acc.ID)
		acc.MFAEnrolled = true
	}
	sess.MFAVerified = true
	return nil
}

// RecoverAccount consumes a one-time recovery code, revokes all old sessions, resets MFA lock, and issues a fresh session.
func (s *Store) RecoverAccount(email, recoveryCode string) (rawToken string, sess *Session, err error) {
	email = strings.ToLower(strings.TrimSpace(email))
	codeHash := hashToken(strings.TrimSpace(recoveryCode))
	s.mu.Lock()
	defer s.mu.Unlock()
	accID, ok := s.byEmail[email]
	if !ok {
		return "", nil, ErrUnauthorized
	}
	codes := s.recoveryCodes[accID]
	matchedIdx := -1
	for i, ch := range codes {
		if ch != "" && subtle.ConstantTimeCompare([]byte(ch), []byte(codeHash)) == 1 {
			matchedIdx = i
			break
		}
	}
	if matchedIdx < 0 {
		return "", nil, ErrUnauthorized
	}
	// Consume code (single-use)
	s.recoveryCodes[accID][matchedIdx] = ""
	// Revoke all existing sessions for this account
	now := s.nowFunc().UTC()
	for _, existing := range s.sessions {
		if existing.AccountID == accID && existing.RevokedAt == nil {
			t := now
			existing.RevokedAt = &t
		}
	}
	rawToken = "ops_" + randomHex(24)
	newSess := &Session{
		ID:          "sess-" + randomHex(8),
		AccountID:   accID,
		TokenHash:   hashToken(rawToken),
		MFAVerified: true,
		ExpiresAt:   now.Add(12 * time.Hour),
		CreatedAt:   now,
	}
	s.sessions[newSess.TokenHash] = newSess
	s.sessionsByID[newSess.ID] = newSess
	return rawToken, newSess, nil
}

// GetPreferences returns preferences for targetAccountID only if actorAccountID == targetAccountID (BOLA check).
func (s *Store) GetPreferences(actorAccountID, targetAccountID string) (*Preferences, error) {
	if actorAccountID == "" {
		return nil, ErrUnauthorized
	}
	if actorAccountID != targetAccountID {
		return nil, ErrForbidden
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.prefs[targetAccountID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *p
	return &cp, nil
}

// UpdatePreferences updates preferences for targetAccountID only if actorAccountID == targetAccountID (BOLA check).
// When RecentSearchesOptIn is set to false, any stored recent searches for the account are immediately purged.
func (s *Store) UpdatePreferences(actorAccountID, targetAccountID string, next Preferences) (*Preferences, error) {
	if actorAccountID == "" {
		return nil, ErrUnauthorized
	}
	if actorAccountID != targetAccountID {
		return nil, ErrForbidden
	}
	switch next.SafeSearch {
	case "strict", "moderate", "off":
	default:
		return nil, ErrInvalidInput
	}
	switch next.Theme {
	case "light", "dark", "system":
	default:
		return nil, ErrInvalidInput
	}
	switch next.Language {
	case "ko", "en":
	default:
		return nil, ErrInvalidInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.prefs[targetAccountID]
	if !ok {
		return nil, ErrNotFound
	}
	p.SafeSearch = next.SafeSearch
	p.Theme = next.Theme
	p.Language = next.Language
	p.PersonalizedSearch = next.PersonalizedSearch
	p.RecentSearchesOptIn = next.RecentSearchesOptIn
	p.SearchCorrectionOptIn = next.SearchCorrectionOptIn
	p.UpdatedAt = s.nowFunc().UTC()
	if !p.RecentSearchesOptIn {
		delete(s.recentSearches, targetAccountID)
	}
	cp := *p
	return &cp, nil
}

// RecordRecentSearch stores a query only if the user has explicitly opted in (RecentSearchesOptIn == true).
func (s *Store) RecordRecentSearch(accountID, normalizedQuery string) {
	if accountID == "" || normalizedQuery == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.prefs[accountID]
	if !ok || !p.RecentSearchesOptIn {
		return
	}
	list := s.recentSearches[accountID]
	filtered := make([]string, 0, len(list)+1)
	filtered = append(filtered, normalizedQuery)
	for _, item := range list {
		if item != normalizedQuery && len(filtered) < 10 {
			filtered = append(filtered, item)
		}
	}
	s.recentSearches[accountID] = filtered
}

// GetRecentSearches returns the user's recent searches only if actorAccountID == targetAccountID.
func (s *Store) GetRecentSearches(actorAccountID, targetAccountID string) ([]string, error) {
	if actorAccountID == "" {
		return nil, ErrUnauthorized
	}
	if actorAccountID != targetAccountID {
		return nil, ErrForbidden
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.prefs[targetAccountID]
	if !ok || !p.RecentSearchesOptIn {
		return []string{}, nil
	}
	list := s.recentSearches[targetAccountID]
	out := make([]string, len(list))
	copy(out, list)
	return out, nil
}

// ClearRecentSearches purges recent searches for targetAccountID if actorAccountID == targetAccountID.
func (s *Store) ClearRecentSearches(actorAccountID, targetAccountID string) error {
	if actorAccountID == "" {
		return ErrUnauthorized
	}
	if actorAccountID != targetAccountID {
		return ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.recentSearches, targetAccountID)
	return nil
}

// SeedOrganizationSite registers a tenant organization, member role, and publisher site for BOLA testing.
func (s *Store) SeedOrganizationSite(orgID, accountID, role, siteID, origin string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.orgMembers[orgID] == nil {
		s.orgMembers[orgID] = make(map[string]string)
	}
	s.orgMembers[orgID][accountID] = role
	if siteID != "" {
		s.sites[siteID] = &Site{
			ID:                siteID,
			OrganizationID:    orgID,
			OwnerID:           accountID,
			Origin:            origin,
			VerificationState: "verified",
			AddedAt:           s.nowFunc().UTC(),
		}
	}
}

// GetOrganizationSite enforces tenant and object-level authorization (BOLA).
func (s *Store) GetOrganizationSite(actorAccountID, orgID, siteID string) (*Site, error) {
	if actorAccountID == "" {
		return nil, ErrUnauthorized
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	members, ok := s.orgMembers[orgID]
	if !ok {
		return nil, ErrForbidden
	}
	if _, isMember := members[actorAccountID]; !isMember {
		return nil, ErrForbidden
	}
	site, ok := s.sites[siteID]
	if !ok || site.OrganizationID != orgID {
		return nil, ErrForbidden
	}
	cp := *site
	return &cp, nil
}

