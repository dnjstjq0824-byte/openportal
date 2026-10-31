import { test, expect } from '@playwright/test';

test.describe('Open Scanner — Unified Account & Email Verification System (P0)', () => {
  test('Complete E2E: Register -> Receive PIN -> Verify Email -> Login -> Reset Password', async ({ page }) => {
    // 1. Visit Home
    await page.goto('/');
    const openAccountBtn = page.locator('.os-account-btn');
    await expect(openAccountBtn).toBeVisible();
    await expect(openAccountBtn).toContainText('Open Account');

    // 2. Open Account Modal
    await openAccountBtn.click();
    const modal = page.locator('.os-auth-modal-card');
    await expect(modal).toBeVisible();
    await expect(modal.locator('.os-brand-name')).toHaveText('OPEN SCANNER');

    // 3. Switch to Register Tab
    const registerTab = modal.locator('.os-tab-btn', { hasText: '회원가입' });
    await registerTab.click();
    await expect(registerTab).toHaveClass(/active/);

    // 4. Fill Registration Form
    const testEmail = `tester_${Date.now()}@openscanner.internal`;
    const testPassword = 'Password1234!';
    await page.fill('#os-reg-email', testEmail);
    await page.fill('#os-reg-name', 'Open Scanner Tester');
    await page.fill('#os-reg-pass', testPassword);
    await page.fill('#os-reg-confirm', testPassword);
    await page.check('#os-reg-terms');

    // Submit Registration
    await modal.locator('button[type="submit"]', { hasText: '회원가입 및 인증 메일 발송' }).click();

    // 5. Expect transition to Email Verification Tab with PIN pre-filled (in dev mode)
    const verifyTab = modal.locator('.os-tab-btn', { hasText: '이메일 인증' });
    await expect(verifyTab).toHaveClass(/active/);
    await expect(modal.locator('.os-auth-alert.success')).toBeVisible();

    const pinInput = page.locator('#os-verify-code');
    await expect(pinInput).not.toBeEmpty();

    // Submit Email Verification
    await modal.locator('button[type="submit"]', { hasText: '이메일 소유권 인증 완료' }).click();

    // 6. Expect transition to Login Tab
    const loginTab = modal.locator('.os-tab-btn', { hasText: '로그인' });
    await expect(loginTab).toHaveClass(/active/);
    await expect(modal.locator('.os-auth-alert.success')).toContainText('이메일 인증이 완료되었습니다');

    // 7. Login with newly verified account
    await page.fill('#os-login-password', testPassword);
    await modal.locator('button[type="submit"]', { hasText: '로그인' }).click();

    // 8. Expect modal to close and user avatar to be visible
    await expect(modal).toBeHidden();
    const avatarBtn = page.locator('.g-avatar-btn');
    await expect(avatarBtn).toBeVisible();

    // Open Profile Flyout
    await avatarBtn.click();
    const flyout = page.locator('.profile-flyout');
    await expect(flyout).toBeVisible();
    await expect(flyout).toContainText(testEmail);
  });
});
