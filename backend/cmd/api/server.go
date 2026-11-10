package main

import (
	"database/sql"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dnjstjq0824-byte/openportal/backend/internal/ads"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/ai"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/email"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/identity"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/launch"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/ops"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/ranking"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/search"
	"github.com/dnjstjq0824-byte/openportal/backend/internal/vertical"
)

type app struct {
	initMu        sync.Mutex
	c             *search.Client
	db            *sql.DB
	idStore       *identity.Store
	emailSvc      *email.Service
	corsOrigin    string
	abuseDetector *ranking.AbuseDetector
	reranker      *ranking.Reranker
	intentClassifier *ranking.IntentClassifier
	safeSearch    *ranking.SafeSearchClassifier
	moderationMgr *ranking.ModerationWorkflowManager
	vertSvc       *vertical.VerticalService
	poiMgr        *vertical.POIManager
	shopMgr       *vertical.ShoppingFeedManager
	aiTracker     *ai.AITelemetryTracker
	adFraud       *ads.ClickFraudDetector
	adLedger      *ads.AdBillingLedger
	wmMgr         *ads.WebmasterManager
	pamMgr        *ops.PAMManager
	dsarMgr       *ops.DSARManager
	dualMgr       *ops.DualCustodyManager
	benchProfiler *launch.LatencyProfile
	drMgr         *launch.DRManager
	sloTracker    *launch.SLOTracker
	defectReg     *launch.DefectRegister
}

type contextKey string

const (
	actorAccountIDKey contextKey = "actor_account_id"
	actorAccountKey   contextKey = "actor_account"
)

func (a *app) ensureStore() *identity.Store {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.idStore == nil {
		a.idStore = identity.NewStore()
	}
	return a.idStore
}

func (a *app) ensureAbuseDetector() *ranking.AbuseDetector {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.abuseDetector == nil {
		a.abuseDetector = ranking.NewAbuseDetector(10*time.Second, 200, 50)
	}
	return a.abuseDetector
}

// redisAddrFromEnv parses REDIS_URL (redis://host:port[/db]) into host:port.
// Returns "" when unset so callers fall back to memory counters.
func redisAddrFromEnv() string {
	raw := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "rediss://") {
		return "" // TLS redis not supported by the minimal RESP client
	}
	raw = strings.TrimPrefix(raw, "redis://")
	if i := strings.Index(raw, "/"); i >= 0 {
		raw = raw[:i]
	}
	if _, _, err := net.SplitHostPort(raw); err != nil {
		return ""
	}
	return raw
}

func (a *app) ensureReranker() *ranking.Reranker {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.reranker == nil {
		a.reranker = ranking.NewReranker(ranking.DefaultRerankerConfig())
	}
	return a.reranker
}

func (a *app) ensureIntentClassifier() *ranking.IntentClassifier {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.intentClassifier == nil {
		a.intentClassifier = ranking.NewIntentClassifier()
	}
	return a.intentClassifier
}

func (a *app) ensureSafeSearch() *ranking.SafeSearchClassifier {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.safeSearch == nil {
		a.safeSearch = ranking.NewSafeSearchClassifier()
	}
	return a.safeSearch
}

func (a *app) ensureModeration() *ranking.ModerationWorkflowManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.moderationMgr == nil {
		a.moderationMgr = ranking.NewModerationWorkflowManager()
	}
	return a.moderationMgr
}

func (a *app) ensureVerticalService() *vertical.VerticalService {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.vertSvc == nil {
		a.vertSvc = vertical.NewVerticalService()
	}
	return a.vertSvc
}

func (a *app) ensurePOIManager() *vertical.POIManager {
	return a.ensureVerticalService().POIManager()
}

func (a *app) ensureShoppingManager() *vertical.ShoppingFeedManager {
	return a.ensureVerticalService().ShoppingManager()
}

func (a *app) ensureAITracker() *ai.AITelemetryTracker {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.aiTracker == nil {
		a.aiTracker = ai.NewAITelemetryTracker(100.0)
	}
	return a.aiTracker
}

func (a *app) ensureWebmaster() *ads.WebmasterManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.wmMgr == nil {
		a.wmMgr = ads.NewWebmasterManager()
	}
	return a.wmMgr
}

func (a *app) ensureAdFraud() *ads.ClickFraudDetector {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.adFraud == nil {
		a.adFraud = ads.NewClickFraudDetector(10 * time.Second)
	}
	return a.adFraud
}

func (a *app) ensureAdLedger() *ads.AdBillingLedger {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.adLedger == nil {
		a.adLedger = ads.NewAdBillingLedger()
		_, _ = a.adLedger.Deposit("adv-portal", 1000000, "KRW", "init-deposit-001")
	}
	return a.adLedger
}

func (a *app) ensureEmailService() *email.Service {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.emailSvc == nil {
		a.emailSvc = email.NewService(email.NewMemorySender(), "no-reply@open-scanner.org")
	}
	return a.emailSvc
}

func (a *app) ensurePAM() *ops.PAMManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.pamMgr == nil {
		a.pamMgr = ops.NewPAMManager(1 * time.Hour)
	}
	return a.pamMgr
}

func (a *app) ensureDSAR() *ops.DSARManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.dsarMgr == nil {
		a.dsarMgr = ops.NewDSARManager()
	}
	return a.dsarMgr
}

func (a *app) ensureDualCustody() *ops.DualCustodyManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.dualMgr == nil {
		a.dualMgr = ops.NewDualCustodyManager()
	}
	return a.dualMgr
}

func (a *app) ensureBenchProfiler() *launch.LatencyProfile {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.benchProfiler == nil {
		a.benchProfiler = launch.NewLatencyProfile()
	}
	return a.benchProfiler
}

func (a *app) ensureDRManager() *launch.DRManager {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.drMgr == nil {
		a.drMgr = launch.NewDRManager()
	}
	return a.drMgr
}

func (a *app) ensureSLOTracker() *launch.SLOTracker {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.sloTracker == nil {
		a.sloTracker = launch.NewSLOTracker()
	}
	return a.sloTracker
}

func (a *app) ensureDefectRegister() *launch.DefectRegister {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if a.defectReg == nil {
		a.defectReg = launch.NewDefectRegister()
	}
	return a.defectReg
}
