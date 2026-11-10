import http from 'node:http';
import { spawn } from 'node:child_process';
import { startStack, listen } from '../tests/helpers/platform-stack.mjs';
import { hashPassword, seal } from '../gateway/security.mjs';
import { buildTransactionalEmail } from '../gateway/mailer.mjs';

const SEARCH_DOCS = [
  {
    title: 'OpenScanner — 정밀 웹 검색, 이메일 및 통합 계정 플랫폼',
    url: 'https://openscanner.com/ko/about',
    site_name: 'OpenScanner',
    description: 'OpenScanner는 정통 웹 검색엔진 아키텍처와 프라이빗 JMAP 이메일, 단일 계정 보안을 결합한 차세대 통합 검색 플랫폼입니다.'
  },
  {
    title: 'OpenScanner 검색 작동 원리 — 하이브리드 색인 및 실시간 랭킹',
    url: 'https://openscanner.com/ko/howsearchworks',
    site_name: 'OpenScanner Engineering',
    description: 'BM25 역색인, Reciprocal Rank Fusion(RRF) 하이브리드 병합, 한국어/영어/태국어 형태소 분석과 안전 검색 필터링 파이프라인을 설명합니다.'
  },
  {
    title: 'OpenScanner Mail — 보안 웹메일 및 첨부파일 실시간 스캔 가이드',
    url: 'https://openscanner.com/ko/mail',
    site_name: 'OpenScanner Mail',
    description: 'JMAP 표준 프로토콜 기반의 빠른 메일함 동기화, ClamAV 첨부파일 악성코드 검사, 엄격한 HTML 살균(Sanitization)으로 피싱과 트래커를 차단합니다.'
  },
  {
    title: 'OpenScanner 계정 보안 및 2단계 이메일 인증(OTP) 설정 안내',
    url: 'https://openscanner.com/ko/account',
    site_name: 'OpenScanner Security',
    description: 'scrypt 패스워드 해싱, 세션 자동 회전(Session Rotation), 6자리 OTP 이메일 인증 및 복구 이메일 검증을 통해 계정을 안전하게 보호하세요.'
  },
  {
    title: 'OpenScanner Search API & OpenSearch XML 브라우저 기본 검색엔진 등록',
    url: 'https://openscanner.com/opensearch.xml',
    site_name: 'OpenScanner Developers',
    description: '브라우저 주소창(Omnibox)에서 바로 OpenScanner 검색과 150ms 실시간 자동완성 제안을 사용할 수 있는 표준 OpenSearch 규격을 지원합니다.'
  },
  {
    title: '123 테스트 및 숫자·코드 정밀 검색 가이드 — OpenScanner 인덱스',
    url: 'https://openscanner.com/ko/docs/numeric-search-123',
    site_name: 'OpenScanner Docs',
    description: '123, 에러 코드, 버전 번호, 포트 번호 및 특수 기호가 포함된 기술 문서와 로그 데이터를 누락 없이 정확하게 탐색하는 방법을 안내합니다.'
  }
];

const SUGGESTIONS = [
  'OpenScanner',
  'OpenScanner 검색',
  'OpenScanner 이메일',
  'OpenScanner 보안 설정',
  '123 테스트 검색',
  '검색엔진 아키텍처',
  '이메일 인증 OTP',
  '하이브리드 검색 RRF'
];

async function startLocalSearchServer(port = 8081) {
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, `http://127.0.0.1:${port}`);
    res.setHeader('Content-Type', 'application/json; charset=utf-8');

    if (url.pathname === '/api/v1/suggest') {
      const q = (url.searchParams.get('q') || '').trim().toLowerCase();
      const matched = SUGGESTIONS.filter(item => item.toLowerCase().includes(q));
      const list = matched.length ? matched : (q ? [`${q} 검색`, `${q} 문서`, `${q} 가이드`, ...SUGGESTIONS.slice(0, 3)] : SUGGESTIONS);
      return res.end(JSON.stringify({ query: q, suggestions: list.slice(0, 8) }));
    }

    if (url.pathname === '/api/v1/search') {
      const q = (url.searchParams.get('q') || '').trim();
      const page = Math.max(1, Number(url.searchParams.get('page') || 1));
      const terms = q.toLowerCase().split(/\s+/).filter(Boolean);
      let hits = SEARCH_DOCS.filter(doc =>
        terms.some(t => doc.title.toLowerCase().includes(t) || doc.description.toLowerCase().includes(t) || doc.url.toLowerCase().includes(t))
      );
      if (hits.length === 0 && q) {
        hits = [
          {
            title: `${q} — OpenScanner 통합 웹 검색 결과`,
            url: `https://openscanner.com/ko/search- reference/${encodeURIComponent(q)}`,
            site_name: 'OpenScanner Index',
            description: `'${q}' 검색어와 일치하는 OpenScanner 로컬 색인 문서입니다. 정밀 키워드 매칭 및 요약 스니펫 하이라이팅이 적용되었습니다.`
          },
          ...SEARCH_DOCS.slice(0, 4)
        ];
      }
      return res.end(
        JSON.stringify({
          query: q,
          total: hits.length,
          page,
          has_more: false,
          elapsed_ms: 12,
          corpus_kind: 'approved-web',
          hits
        })
      );
    }

    res.statusCode = 404;
    res.end(JSON.stringify({ error: 'not_found' }));
  });

  await listen(server, port);
  return server;
}

async function seedAccounts(stack) {
  const passwordPlain = 'OpenScanner!2026';
  const passwordHash = await hashPassword(passwordPlain);

  const accounts = [
    {
      username: 'sysadmin',
      email: 'sysadmin@openscanner.test',
      displayName: 'OpenScanner 관리자',
      recoveryEmail: 'sysadmin.recovery@example.com',
      isAdmin: true
    },
    {
      username: 'tester01',
      email: 'tester01@openscanner.test',
      displayName: '홍길동',
      recoveryEmail: 'tester01.recovery@example.com',
      isAdmin: false
    }
  ];

  for (const acc of accounts) {
    const res = await stack.pool.query(
      `INSERT INTO webapp.accounts (
        email, username, password_hash, display_name, given_name, family_name,
        birth_date, gender, recovery_email, recovery_phone, locale,
        verified_at, recovery_email_verified_at, is_admin
      ) VALUES (
        $1, $2, $3, $4, '길동', '홍',
        '1992-03-15', 'unspecified', $5, '+821012345678', 'ko',
        now(), now(), $6
      )
      ON CONFLICT (email) DO UPDATE SET
        password_hash = EXCLUDED.password_hash,
        is_admin = EXCLUDED.is_admin,
        verified_at = now()
      RETURNING id`,
      [acc.email, acc.username, passwordHash, acc.displayName, acc.recoveryEmail, acc.isAdmin]
    );
    const accountId = res.rows[0].id;
    const mailbox = stack.jmap.provision(acc.email);
    await stack.pool.query(
      `INSERT INTO webapp.mail_connections (account_id, token_ciphertext)
       VALUES ($1, $2)
       ON CONFLICT (account_id) DO UPDATE SET token_ciphertext = EXCLUDED.token_ciphertext`,
      [accountId, seal(mailbox.token, stack.config.mailKey)]
    );

    const welcomeMail = buildTransactionalEmail('welcome', {
      locale: 'ko',
      displayName: acc.displayName,
      email: acc.email,
      link: 'http://localhost:5173/ko'
    });
    const verifyMail = buildTransactionalEmail('verify', {
      locale: 'ko',
      token: 'sample-verify-token-2026',
      link: 'http://localhost:5173/ko/verify-email#token=sample-verify-token-2026'
    });
    const securityMail = buildTransactionalEmail('security', {
      locale: 'ko',
      action: '새 기기 로그인 및 보안 점검 완료',
      timestamp: new Date().toISOString()
    });

    stack.jmap.receive(mailbox.token, {
      from: [{ name: 'OpenScanner', email: 'no-reply@openscanner.com' }],
      to: [{ name: acc.displayName, email: acc.email }],
      subject: welcomeMail.subject,
      bodyValues: {
        text: { value: welcomeMail.text },
        html: { value: `<p><strong>${acc.displayName}</strong>님, OpenScanner 가입을 환영합니다.</p><p>${welcomeMail.text}</p>` }
      }
    });
    stack.jmap.receive(mailbox.token, {
      from: [{ name: 'OpenScanner Security', email: 'no-reply@openscanner.com' }],
      to: [{ name: acc.displayName, email: acc.email }],
      subject: `${verifyMail.subject} (OTP: ${verifyMail.otpCode})`,
      bodyValues: {
        text: { value: verifyMail.text },
        html: { value: `<p>이메일 인증번호(OTP): <strong>${verifyMail.otpCode}</strong></p><p>본 인증번호는 30분 동안 유효합니다.</p>` }
      }
    });
    stack.jmap.receive(mailbox.token, {
      from: [{ name: 'OpenScanner Security', email: 'security@openscanner.com' }],
      to: [{ name: acc.displayName, email: acc.email }],
      subject: securityMail.subject,
      bodyValues: {
        text: { value: securityMail.text },
        html: { value: `<p>${securityMail.text}</p>` }
      }
    });
  }
}

async function main() {
  const searchServer = await startLocalSearchServer(0);
  const searchPort = searchServer.address().port;
  const stack = await startStack({
    origin: 'http://localhost:5173',
    gatewayPort: 0,
    pgPort: 55465,
    searchURL: `http://127.0.0.1:${searchPort}`,
    hostedMail: true
  });
  const gatewayPort = stack.gateway.server.address().port;

  await seedAccounts(stack);

  const viteProc = spawn('npm', ['run', 'dev', '--prefix', 'apps/web', '--', '--host', '0.0.0.0', '--port', '5173'], {
    stdio: 'inherit',
    shell: true,
    env: {
      ...process.env,
      GATEWAY_PROXY_TARGET: `http://127.0.0.1:${gatewayPort}`
    }
  });

  console.log('====================================================');
  console.log('OpenScanner Local Stack Ready!');
  console.log('Frontend URL: http://localhost:5173/ko');
  console.log(`Gateway URL:  http://127.0.0.1:${gatewayPort}`);
  console.log(`Search URL:   http://127.0.0.1:${searchPort}`);
  console.log('Admin Login:  sysadmin / OpenScanner!2026');
  console.log('User Login:   tester01 / OpenScanner!2026');
  console.log('====================================================');

  const shutdown = async () => {
    viteProc.kill();
    await stack.close();
    await new Promise(r => searchServer.close(r));
    process.exit(0);
  };
  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
