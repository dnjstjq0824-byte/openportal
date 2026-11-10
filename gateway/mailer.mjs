import { createHash } from 'node:crypto';
import nodemailer from 'nodemailer';
import { emailText } from './translations.mjs';

const ESCAPE_MAP = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;'
};

function escapeHtml(value = '') {
  return String(value).replace(/[&<>"']/g, ch => ESCAPE_MAP[ch]);
}

/**
 * RFC 2047 UTF-8 Base64 MIME word encoding for non-ASCII header tokens.
 */
export function encodeMimeWord(value = '') {
  const str = String(value).trim();
  if (!str) return '';
  if (/^[\x20-\x7E]+$/.test(str)) return str;
  return `=?UTF-8?B?${Buffer.from(str, 'utf8').toString('base64')}?=`;
}

/**
 * Formats a From header with RFC 2047 UTF-8 encoding when needed.
 */
export function formatFromHeader({ from, fromName = 'OpenScanner', fromEmail = 'no-reply@openscanner.com' } = {}) {
  if (from && String(from).trim()) {
    const raw = String(from).trim();
    const match = raw.match(/^"?([^"<]+?)"?\s*<([^>]+)>$/);
    if (match) {
      const [, name, addr] = match;
      return `${encodeMimeWord(name.trim())} <${addr.trim()}>`;
    }
    return raw;
  }
  const cleanEmail = String(fromEmail || 'no-reply@openscanner.com').trim();
  const cleanName = String(fromName || 'OpenScanner').trim();
  return `${encodeMimeWord(cleanName)} <${cleanEmail}>`;
}

/**
 * Derives a deterministic 6-digit numeric OTP from a verification token.
 */
export function deriveNumericOtp(token = '') {
  const digest = createHash('sha256').update(String(token)).digest();
  const num = digest.readUInt32BE(0) % 1000000;
  return String(num).padStart(6, '0');
}

const COPY = {
  ko: {
    verifySubject: '[OpenScanner] 이메일 인증을 완료해 주세요',
    verifyTitle: '이메일 주소를 인증해 주세요',
    verifyIntro: 'OpenScanner 계정을 안전하게 보호하기 위해 아래 6자리 인증번호를 입력하거나 인증 버튼을 클릭해 주세요.',
    verifyOtpLabel: '이메일 인증번호 (6자리)',
    verifyCta: '이메일 주소 인증하기',
    verifyFallback: '버튼이 작동하지 않으면 아래 링크를 복사하여 브라우저 주소창에 붙여넣으세요.',
    verifyExpiry: '본 인증 링크와 인증번호는 발송 시점으로부터 30분 동안 유효합니다.',
    resetSubject: '[OpenScanner] 비밀번호 재설정 안내',
    resetTitle: '비밀번호 재설정 요청이 접수되었습니다',
    resetIntro: '아래 버튼을 클릭하여 OpenScanner 계정의 새 비밀번호를 설정해 주세요.',
    resetCta: '비밀번호 재설정하기',
    resetFallback: '버튼이 작동하지 않으면 아래 링크를 복사하여 브라우저 주소창에 붙여넣으세요.',
    resetSecurity: '본인이 비밀번호 재설정을 요청하지 않았다면 이 이메일을 무시해 주세요. 기존 비밀번호는 안전하게 유지됩니다.',
    resetExpiry: '보안을 위해 본 재설정 링크는 30분 후 만료됩니다.',
    welcomeSubject: '[OpenScanner] 가입을 환영합니다',
    welcomeTitle: 'OpenScanner에 오신 것을 환영합니다',
    welcomeIntro: '계정 생성이 완료되었습니다. 하나의 OpenScanner 계정으로 정밀 웹 검색, 개인 메일함, 보안 설정을 즉시 이용하실 수 있습니다.',
    welcomeCta: 'OpenScanner 시작하기',
    securitySubject: '[OpenScanner] 계정 보안 알림',
    securityTitle: '계정 보안 설정이 변경되었습니다',
    securityIntro: '회원님의 OpenScanner 계정에서 다음과 같은 보안 작업이 수행되었습니다.',
    securityActionLabel: '수행된 작업',
    securityTimeLabel: '발생 시각 (UTC)',
    securityWarning: '본인이 수행한 작업이 아니라면 즉시 비밀번호를 재설정하고 고객지원팀으로 문의해 주세요.',
    footerNote: '본 메일은 발신 전용입니다. 문의 사항은 고객지원 채널을 이용해 주세요.',
    copyright: '© OpenScanner. All rights reserved.'
  },
  en: {
    verifySubject: '[OpenScanner] Verify your email address',
    verifyTitle: 'Verify your email address',
    verifyIntro: 'To secure your OpenScanner account, enter the 6-digit verification code below or click the verification button.',
    verifyOtpLabel: 'Verification code (6 digits)',
    verifyCta: 'Verify email address',
    verifyFallback: 'If the button does not work, copy and paste the link below into your browser:',
    verifyExpiry: 'This verification link and code expire in 30 minutes.',
    resetSubject: '[OpenScanner] Reset your password',
    resetTitle: 'Password reset request received',
    resetIntro: 'Click the button below to choose a new password for your OpenScanner account.',
    resetCta: 'Reset password',
    resetFallback: 'If the button does not work, copy and paste the link below into your browser:',
    resetSecurity: 'If you did not request a password reset, you can safely ignore this email. Your password will remain unchanged.',
    resetExpiry: 'For your security, this reset link expires in 30 minutes.',
    welcomeSubject: '[OpenScanner] Welcome to OpenScanner',
    welcomeTitle: 'Welcome to OpenScanner',
    welcomeIntro: 'Your account is ready. Access precision web search, private mail, and unified security controls with a single OpenScanner account.',
    welcomeCta: 'Open OpenScanner',
    securitySubject: '[OpenScanner] Account security alert',
    securityTitle: 'Security activity on your account',
    securityIntro: 'The following security action was completed on your OpenScanner account:',
    securityActionLabel: 'Action',
    securityTimeLabel: 'Timestamp (UTC)',
    securityWarning: 'If you did not authorize this change, reset your password immediately and contact support.',
    footerNote: 'This is an automated transactional message from OpenScanner.',
    copyright: '© OpenScanner. All rights reserved.'
  },
  th: {
    verifySubject: '[OpenScanner] ยืนยันที่อยู่อีเมลของคุณ',
    verifyTitle: 'ยืนยันที่อยู่อีเมลของคุณ',
    verifyIntro: 'เพื่อความปลอดภัยของบัญชี OpenScanner โปรดกรอกรหัสยืนยัน 6 หลักด้านล่าง หรือคลิกปุ่มยืนยัน',
    verifyOtpLabel: 'รหัสยืนยัน (6 หลัก)',
    verifyCta: 'ยืนยันที่อยู่อีเมล',
    verifyFallback: 'หากปุ่มไม่ทำงาน โปรดคัดลอกและวางลิงก์ด้านล่างในเบราว์เซอร์ของคุณ:',
    verifyExpiry: 'ลิงก์และรหัสยืนยันนี้จะหมดอายุภายใน 30 นาที',
    resetSubject: '[OpenScanner] ตั้งรหัสผ่านใหม่',
    resetTitle: 'ได้รับคำขอตั้งรหัสผ่านใหม่',
    resetIntro: 'คลิกปุ่มด้านล่างเพื่อตั้งรหัสผ่านใหม่สำหรับบัญชี OpenScanner ของคุณ',
    resetCta: 'ตั้งรหัสผ่านใหม่',
    resetFallback: 'หากปุ่มไม่ทำงาน โปรดคัดลอกและวางลิงก์ด้านล่างในเบราว์เซอร์ของคุณ:',
    resetSecurity: 'หากคุณไม่ได้ร้องขอการตั้งรหัสผ่านใหม่ โปรดละเว้นอีเมลนี้ รหัสผ่านเดิมของคุณยังคงปลอดภัย',
    resetExpiry: 'เพื่อความปลอดภัย ลิงก์นี้จะหมดอายุภายใน 30 นาที',
    welcomeSubject: '[OpenScanner] ยินดีต้อนรับสู่ OpenScanner',
    welcomeTitle: 'ยินดีต้อนรับสู่ OpenScanner',
    welcomeIntro: 'บัญชีของคุณพร้อมใช้งานแล้ว เข้าถึงการค้นหาเว็บ อีเมลส่วนตัว และการตั้งค่าความปลอดภัยได้ในที่เดียว',
    welcomeCta: 'เริ่มต้นใช้งาน OpenScanner',
    securitySubject: '[OpenScanner] แจ้งเตือนความปลอดภัยของบัญชี',
    securityTitle: 'มีการเปลี่ยนแปลงความปลอดภัยของบัญชี',
    securityIntro: 'มีการดำเนินการด้านความปลอดภัยต่อไปนี้ในบัญชี OpenScanner ของคุณ:',
    securityActionLabel: 'การดำเนินการ',
    securityTimeLabel: 'เวลา (UTC)',
    securityWarning: 'หากคุณไม่ได้ดำเนินการนี้ โปรดตั้งรหัสผ่านใหม่ทันทีและติดต่อฝ่ายสนับสนุน',
    footerNote: 'อีเมลฉบับนี้เป็นการแจ้งเตือนอัตโนมัติจากระบบ OpenScanner',
    copyright: '© OpenScanner. All rights reserved.'
  }
};

function renderTableLayout({ lang = 'ko', title, bodyHtml, supportEmail = 'support@openscanner.com' }) {
  const c = COPY[lang] || COPY.en;
  return `<!DOCTYPE html>
<html lang="${escapeHtml(lang)}">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>${escapeHtml(title)}</title>
</head>
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:'Pretendard Variable',Pretendard,-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#0F172A;-webkit-text-size-adjust:100%;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color:#F8FAFC;padding:32px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="max-width:600px;background-color:#FFFFFF;border:1px solid #E2E8F0;border-radius:12px;overflow:hidden;">
          <!-- Header -->
          <tr>
            <td style="padding:24px 32px;border-bottom:1px solid #E2E8F0;background-color:#FFFFFF;">
              <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
                <tr>
                  <td align="left" style="font-size:20px;line-height:1.2;letter-spacing:-0.03em;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
                    <span style="display:inline-block;padding:3px 7px;margin-right:6px;border:1.5px solid #0F172A;border-radius:6px;color:#1D4ED8;font-weight:800;font-size:13px;">[&nbsp;&bull;&nbsp;]</span><span style="font-weight:600;color:#0F172A;">Open</span><span style="font-weight:800;color:#1D4ED8;">Scanner</span>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Main Content -->
          <tr>
            <td style="padding:32px;">
              <h1 style="margin:0 0 12px 0;font-size:20px;line-height:1.35;font-weight:700;color:#0F172A;">${escapeHtml(title)}</h1>
              ${bodyHtml}
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding:20px 32px;background-color:#F8FAFC;border-top:1px solid #E2E8F0;font-size:12px;line-height:1.6;color:#475569;">
              <p style="margin:0 0 4px 0;">${escapeHtml(c.footerNote)} (${escapeHtml(supportEmail)})</p>
              <p style="margin:0;color:#94A3B8;font-weight:600;">${escapeHtml(c.copyright)}</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`;
}

/**
 * Builds one of the 4 official OpenScanner transactional email templates:
 * - 'verify': Email verification (6-digit OTP + one-click link)
 * - 'reset': Password reset (CTA button + fallback URL + security notice)
 * - 'welcome': Account welcome notification
 * - 'security': Security alert notification
 */
export function buildTransactionalEmail(purpose, {
  locale = 'ko',
  link = '',
  token = '',
  otpCode = '',
  displayName = '',
  email = '',
  action = 'Password updated',
  timestamp = new Date().toISOString(),
  supportEmail = 'support@openscanner.com'
} = {}) {
  const lang = COPY[locale] ? locale : 'en';
  const c = COPY[lang];
  const legacyText = emailText[lang] || emailText.en;

  if (purpose === 'verify') {
    const code = otpCode || deriveNumericOtp(token || link);
    const bodyHtml = `
      <p style="margin:0 0 20px 0;font-size:15px;line-height:1.6;color:#475569;">${escapeHtml(c.verifyIntro)}</p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 24px 0;">
        <tr>
          <td align="center" style="padding:18px 16px;background-color:#EFF6FF;border:1px solid #BFDBFE;border-radius:10px;">
            <div style="font-size:12px;font-weight:700;color:#475569;margin-bottom:6px;text-transform:uppercase;letter-spacing:0.06em;">${escapeHtml(c.verifyOtpLabel)}</div>
            <div style="font-size:28px;font-weight:800;letter-spacing:6px;color:#1D4ED8;font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace;">${escapeHtml(code)}</div>
          </td>
        </tr>
      </table>
      <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 24px 0;">
        <tr>
          <td align="center" bgcolor="#1D4ED8" style="border-radius:8px;">
            <a href="${escapeHtml(link)}" target="_blank" rel="noopener noreferrer" style="display:inline-block;padding:13px 24px;font-size:15px;font-weight:700;color:#FFFFFF;text-decoration:none;border-radius:8px;background-color:#1D4ED8;">${escapeHtml(c.verifyCta)}</a>
          </td>
        </tr>
      </table>
      <p style="margin:0 0 6px 0;font-size:13px;line-height:1.5;color:#475569;">${escapeHtml(c.verifyFallback)}</p>
      <p style="margin:0 0 16px 0;font-size:12px;line-height:1.5;word-break:break-all;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;"><a href="${escapeHtml(link)}" style="color:#1D4ED8;text-decoration:underline;">${escapeHtml(link)}</a></p>
      <p style="margin:0;font-size:12px;line-height:1.5;color:#94A3B8;">${escapeHtml(c.verifyExpiry)}</p>
    `;
    const text = `${legacyText.verifyBody}\n\nOTP: ${code}\n${link}\n\n${c.copyright}`;
    return {
      subject: c.verifySubject,
      text,
      html: renderTableLayout({ lang, title: c.verifyTitle, bodyHtml, supportEmail }),
      otpCode: code
    };
  }

  if (purpose === 'reset') {
    const bodyHtml = `
      <p style="margin:0 0 24px 0;font-size:15px;line-height:1.6;color:#475569;">${escapeHtml(c.resetIntro)}</p>
      <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 24px 0;">
        <tr>
          <td align="center" bgcolor="#1D4ED8" style="border-radius:8px;">
            <a href="${escapeHtml(link)}" target="_blank" rel="noopener noreferrer" style="display:inline-block;padding:13px 24px;font-size:15px;font-weight:700;color:#FFFFFF;text-decoration:none;border-radius:8px;background-color:#1D4ED8;">${escapeHtml(c.resetCta)}</a>
          </td>
        </tr>
      </table>
      <p style="margin:0 0 6px 0;font-size:13px;line-height:1.5;color:#475569;">${escapeHtml(c.resetFallback)}</p>
      <p style="margin:0 0 18px 0;font-size:12px;line-height:1.5;word-break:break-all;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;"><a href="${escapeHtml(link)}" style="color:#1D4ED8;text-decoration:underline;">${escapeHtml(link)}</a></p>
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 12px 0;">
        <tr>
          <td style="padding:12px 14px;background-color:#FFFBEB;border:1px solid #FDE68A;border-radius:8px;font-size:13px;line-height:1.5;color:#B45309;">
            ${escapeHtml(c.resetSecurity)}
          </td>
        </tr>
      </table>
      <p style="margin:0;font-size:12px;line-height:1.5;color:#94A3B8;">${escapeHtml(c.resetExpiry)}</p>
    `;
    const text = `${legacyText.resetBody}\n\n${link}\n\n${c.copyright}`;
    return {
      subject: c.resetSubject,
      text,
      html: renderTableLayout({ lang, title: c.resetTitle, bodyHtml, supportEmail })
    };
  }

  if (purpose === 'welcome') {
    const targetLink = link || 'https://openportal-a94.pages.dev/ko';
    const greeting = displayName ? `${displayName} (${email || 'OpenScanner'})` : (email || 'OpenScanner');
    const bodyHtml = `
      <p style="margin:0 0 12px 0;font-size:15px;font-weight:600;color:#0F172A;">${escapeHtml(greeting)}</p>
      <p style="margin:0 0 24px 0;font-size:15px;line-height:1.6;color:#475569;">${escapeHtml(c.welcomeIntro)}</p>
      <table role="presentation" cellpadding="0" cellspacing="0" border="0">
        <tr>
          <td align="center" bgcolor="#1D4ED8" style="border-radius:8px;">
            <a href="${escapeHtml(targetLink)}" target="_blank" rel="noopener noreferrer" style="display:inline-block;padding:13px 24px;font-size:15px;font-weight:700;color:#FFFFFF;text-decoration:none;border-radius:8px;background-color:#1D4ED8;">${escapeHtml(c.welcomeCta)}</a>
          </td>
        </tr>
      </table>
    `;
    const text = `${c.welcomeTitle}\n${greeting}\n\n${c.welcomeIntro}\n\n${targetLink}\n\n${c.copyright}`;
    return {
      subject: c.welcomeSubject,
      text,
      html: renderTableLayout({ lang, title: c.welcomeTitle, bodyHtml, supportEmail })
    };
  }

  // Default: 'security'
  const bodyHtml = `
    <p style="margin:0 0 18px 0;font-size:15px;line-height:1.6;color:#475569;">${escapeHtml(c.securityIntro)}</p>
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="margin:0 0 18px 0;border:1px solid #E2E8F0;border-radius:8px;background-color:#F8FAFC;">
      <tr>
        <td style="padding:10px 14px;border-bottom:1px solid #E2E8F0;font-size:13px;color:#475569;width:140px;">${escapeHtml(c.securityActionLabel)}</td>
        <td style="padding:10px 14px;border-bottom:1px solid #E2E8F0;font-size:13px;font-weight:700;color:#0F172A;">${escapeHtml(action)}</td>
      </tr>
      <tr>
        <td style="padding:10px 14px;font-size:13px;color:#475569;">${escapeHtml(c.securityTimeLabel)}</td>
        <td style="padding:10px 14px;font-size:13px;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;color:#0F172A;">${escapeHtml(timestamp)}</td>
      </tr>
    </table>
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
      <tr>
        <td style="padding:12px 14px;background-color:#FEF2F2;border:1px solid #FECACA;border-radius:8px;font-size:13px;line-height:1.5;color:#B91C1C;">
          ${escapeHtml(c.securityWarning)}
        </td>
      </tr>
    </table>
  `;
  const text = `${c.securityTitle}\n\n${c.securityIntro}\n- ${c.securityActionLabel}: ${action}\n- ${c.securityTimeLabel}: ${timestamp}\n\n${c.securityWarning}\n\n${c.copyright}`;
  return {
    subject: c.securitySubject,
    text,
    html: renderTableLayout({ lang, title: c.securityTitle, bodyHtml, supportEmail })
  };
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * Creates a Dual-Mode Mailer:
 * - Mode A ('smtp'): Live Nodemailer SMTP transport with exponential backoff retry (up to 2 retries).
 * - Mode B ('preview'): Pre-Contract Fallback & Dev Preview mode that logs structured preview output and records messages in memory.
 * - Returns null when SMTP is unconfigured and preview mode is not enabled (preserving strict 503 EMAIL_NOT_CONFIGURED semantics in tests).
 */
export function createDualModeMailer(config = {}) {
  const mode = String(config.smtpMode || 'auto').toLowerCase();
  const hasHost = Boolean(config.smtpHost && String(config.smtpHost).trim());

  const useLiveSmtp = mode === 'smtp' || (mode === 'auto' && hasHost);
  const usePreview = mode === 'preview' || (mode === 'auto' && !hasHost && config.env !== 'test' && config.enablePreviewFallback);

  if (!useLiveSmtp && !usePreview) {
    return null;
  }

  const fromHeader = formatFromHeader({
    from: config.smtpFrom,
    fromName: config.smtpFromName || 'OpenScanner',
    fromEmail: config.smtpFromEmail || 'no-reply@openscanner.com'
  });
  const replyTo = config.smtpReplyTo || 'support@openscanner.com';
  const previews = [];

  if (useLiveSmtp) {
    const transport = nodemailer.createTransport({
      host: config.smtpHost,
      port: Number(config.smtpPort) || 587,
      secure: Boolean(config.smtpSecure) || Number(config.smtpPort) === 465,
      pool: config.smtpPoolEnabled ? true : false,
      connectionTimeout: Number(config.smtpConnectionTimeoutMs) || 10000,
      auth: config.smtpUser ? { user: config.smtpUser, pass: config.smtpPass } : undefined
    });

    return {
      mode: 'smtp',
      previews,
      async sendMail(options) {
        const mailOptions = {
          from: options.from || fromHeader,
          replyTo: options.replyTo || replyTo,
          textEncoding: 'quoted-printable',
          ...options
        };
        let lastErr;
        const maxRetries = Number.isInteger(config.smtpMaxRetries) ? config.smtpMaxRetries : 2;
        for (let attempt = 0; attempt <= maxRetries; attempt++) {
          try {
            return await transport.sendMail(mailOptions);
          } catch (err) {
            lastErr = err;
            if (attempt < maxRetries) {
              await sleep(120 * Math.pow(2, attempt));
            }
          }
        }
        throw lastErr;
      }
    };
  }

  return {
    mode: 'preview',
    previews,
    async sendMail(options) {
      const entry = {
        id: `preview-${Date.now()}-${previews.length + 1}`,
        from: options.from || fromHeader,
        replyTo: options.replyTo || replyTo,
        to: options.to,
        subject: options.subject,
        text: options.text || '',
        html: options.html || '',
        createdAt: new Date().toISOString()
      };
      previews.push(entry);
      if (previews.length > 50) previews.shift();
      if (config.env !== 'test') {
        console.info(
          `\n┌──────────────────────────────────────────────────────────────┐\n` +
          `│ [OpenScanner Mailer — PREVIEW MODE (No SMTP_HOST Configured)] │\n` +
          `├──────────────────────────────────────────────────────────────┤\n` +
          `│ To:      ${String(entry.to).slice(0, 50).padEnd(50)} │\n` +
          `│ Subject: ${String(entry.subject).slice(0, 50).padEnd(50)} │\n` +
          `└──────────────────────────────────────────────────────────────┘`
        );
      }
      return { messageId: entry.id, preview: true };
    }
  };
}
