import type { Locale } from './i18n';

const en = {
  steps: 'Step {n} of {total}', back: 'Back', next: 'Next', edit: 'Edit',
  nameTitle: 'Create an OpenScanner account', nameIntro: 'Enter your name.', familyName: 'Last name (optional)', givenName: 'First name',
  basicTitle: 'Basic information', basicIntro: 'Enter your birthday and gender.', birthday: 'Birthday', year: 'Year', month: 'Month', day: 'Day',
  gender: 'Gender', genderSelect: 'Select', female: 'Female', male: 'Male', unspecified: 'Rather not say', custom: 'Custom',
  genderCustom: 'What is your gender?', pronoun: 'Please refer to me as', pronounFemale: 'Female', pronounMale: 'Male', pronounOther: 'Other',
  ageHint: 'Your birthday is used to confirm you meet the minimum age. It is not shown to other people.', underAge: 'You must be at least 14 years old to create an account. Accounts for younger users require guardian consent, which is not available yet.', invalidDate: 'Enter a valid date.',
  usernameTitle: 'Choose your email address', usernameIntro: 'This becomes your OpenScanner mail address and sign-in ID.', username: 'Username',
  usernameRule: '6–30 characters: lowercase letters, numbers and periods. Must start and end with a letter or number.', checking: 'Checking availability…',
  available: 'Available', taken: 'That username is taken. Try another.', format: 'Use 6–30 lowercase letters, numbers or periods (no ".." and no period at the start or end).', reserved: 'This username is reserved.', suggestions: 'Available:',
  passwordTitle: 'Create a strong password', passwordIntro: 'Use at least 12 characters. A mix of letters, numbers and symbols is stronger.',
  recoveryTitle: 'Add recovery information', recoveryIntro: 'Used to recover your account if you forget your password.', recoveryEmail: 'Recovery email (optional)', recoveryEmailHint: 'Use an address at another provider. We send a confirmation link to it.',
  phone: 'Phone number (optional)', country: 'Country code', phoneHint: 'Stored for account recovery. SMS verification is not available yet, so the number stays unverified.', recoveryWarning: 'Without a confirmed recovery email, a forgotten password cannot be reset.',
  reviewTitle: 'Review your account', reviewIntro: 'Check your information before creating the account.', name: 'Name', address: 'Email address', none: 'Not provided',
  create: 'Create account', creating: 'Creating your account…',
  doneTitle: 'Your account is ready', doneIntro: 'Your mailbox was created and you are signed in.', doneRecoverySent: 'A confirmation link was sent to your recovery email.', doneRecoveryNotSent: 'The recovery email confirmation could not be sent yet. You can resend it from your account page.', openMail: 'Open mailbox', goHome: 'Go to home',
  unavailable: 'New mailbox accounts cannot be created right now. Please try again later.',
  loginId: 'Email or username', loginIntro: 'Sign in with your OpenScanner username or email address.', forgotIntro: 'If the account has a confirmed recovery email, we will send a reset link there.',
  recoveryStatus: 'Recovery email', verified: 'Confirmed', unverified: 'Not confirmed', resendRecovery: 'Send confirmation again', mailAddress: 'Mail address',
};
type SignupCopy = { [K in keyof typeof en]: string };
const ko: SignupCopy = {
  steps: '{total}단계 중 {n}단계', back: '이전', next: '다음', edit: '변경',
  nameTitle: 'OpenScanner 계정 만들기', nameIntro: '이름을 입력하세요.', familyName: '성(선택사항)', givenName: '이름',
  basicTitle: '기본 정보', basicIntro: '생년월일과 성별을 입력하세요.', birthday: '생년월일', year: '연도', month: '월', day: '일',
  gender: '성별', genderSelect: '선택', female: '여성', male: '남성', unspecified: '밝히고 싶지 않음', custom: '직접 입력',
  genderCustom: '성별을 입력하세요', pronoun: '호칭', pronounFemale: '여성', pronounMale: '남성', pronounOther: '기타',
  ageHint: '생년월일은 최소 가입 연령 확인에 사용하며 다른 사람에게 표시되지 않습니다.', underAge: '만 14세 이상만 가입할 수 있습니다. 만 14세 미만은 법정대리인 동의가 필요하며 이 기능은 아직 제공하지 않습니다.', invalidDate: '올바른 날짜를 입력하세요.',
  usernameTitle: '이메일 주소 만들기', usernameIntro: 'OpenScanner 메일 주소이자 로그인 아이디로 사용됩니다.', username: '사용자 이름',
  usernameRule: '6~30자의 영문 소문자, 숫자, 마침표를 사용할 수 있으며 영문 또는 숫자로 시작하고 끝나야 합니다.', checking: '사용 가능 여부 확인 중…',
  available: '사용할 수 있습니다', taken: '이미 사용 중인 이름입니다. 다른 이름을 입력하세요.', format: '6~30자의 영문 소문자, 숫자, 마침표만 사용하세요("..", 앞뒤 마침표 불가).', reserved: '예약된 이름이라 사용할 수 없습니다.', suggestions: '사용 가능:',
  passwordTitle: '안전한 비밀번호 만들기', passwordIntro: '12자 이상 입력하세요. 영문·숫자·기호를 섞으면 더 안전합니다.',
  recoveryTitle: '복구 정보 추가', recoveryIntro: '비밀번호를 잊었을 때 계정을 복구하는 데 사용합니다.', recoveryEmail: '복구 이메일(선택사항)', recoveryEmailHint: '다른 메일 서비스의 주소를 입력하세요. 확인 링크를 보내드립니다.',
  phone: '전화번호(선택사항)', country: '국가 번호', phoneHint: '계정 복구용으로 저장합니다. 문자(SMS) 인증은 아직 지원하지 않아 미인증 상태로 남습니다.', recoveryWarning: '확인된 복구 이메일이 없으면 비밀번호를 잊었을 때 재설정할 수 없습니다.',
  reviewTitle: '계정 정보 확인', reviewIntro: '계정을 만들기 전에 입력한 정보를 확인하세요.', name: '이름', address: '이메일 주소', none: '입력 안 함',
  create: '계정 만들기', creating: '계정을 만드는 중…',
  doneTitle: '계정이 만들어졌습니다', doneIntro: '메일함이 생성되었고 로그인되었습니다.', doneRecoverySent: '복구 이메일로 확인 링크를 보냈습니다.', doneRecoveryNotSent: '복구 이메일 확인 메일을 아직 보내지 못했습니다. 계정 화면에서 다시 보낼 수 있습니다.', openMail: '메일함 열기', goHome: '홈으로',
  unavailable: '지금은 새 메일 계정을 만들 수 없습니다. 잠시 후 다시 시도하세요.',
  loginId: '이메일 또는 아이디', loginIntro: 'OpenScanner 아이디 또는 이메일 주소로 로그인하세요.', forgotIntro: '확인된 복구 이메일이 있는 계정이면 그 주소로 재설정 링크를 보냅니다.',
  recoveryStatus: '복구 이메일', verified: '확인됨', unverified: '확인 필요', resendRecovery: '확인 메일 다시 보내기', mailAddress: '메일 주소',
};
const th: SignupCopy = {
  steps: 'ขั้นตอนที่ {n} จาก {total}', back: 'ย้อนกลับ', next: 'ถัดไป', edit: 'แก้ไข',
  nameTitle: 'สร้างบัญชี OpenScanner', nameIntro: 'ป้อนชื่อของคุณ', familyName: 'นามสกุล (ไม่บังคับ)', givenName: 'ชื่อ',
  basicTitle: 'ข้อมูลพื้นฐาน', basicIntro: 'ป้อนวันเกิดและเพศของคุณ', birthday: 'วันเกิด', year: 'ปี', month: 'เดือน', day: 'วัน',
  gender: 'เพศ', genderSelect: 'เลือก', female: 'หญิง', male: 'ชาย', unspecified: 'ไม่ต้องการระบุ', custom: 'กำหนดเอง',
  genderCustom: 'เพศของคุณคืออะไร', pronoun: 'โปรดเรียกฉันว่า', pronounFemale: 'หญิง', pronounMale: 'ชาย', pronounOther: 'อื่นๆ',
  ageHint: 'ใช้วันเกิดเพื่อยืนยันอายุขั้นต่ำและจะไม่แสดงต่อผู้อื่น', underAge: 'ต้องมีอายุอย่างน้อย 14 ปีจึงจะสร้างบัญชีได้ ผู้ที่อายุน้อยกว่าต้องได้รับความยินยอมจากผู้ปกครอง ซึ่งยังไม่รองรับ', invalidDate: 'ป้อนวันที่ที่ถูกต้อง',
  usernameTitle: 'สร้างที่อยู่อีเมลของคุณ', usernameIntro: 'จะใช้เป็นที่อยู่อีเมล OpenScanner และ ID สำหรับเข้าสู่ระบบ', username: 'ชื่อผู้ใช้',
  usernameRule: '6–30 ตัวอักษร: ตัวพิมพ์เล็ก ตัวเลข และจุด ต้องขึ้นต้นและลงท้ายด้วยตัวอักษรหรือตัวเลข', checking: 'กำลังตรวจสอบ…',
  available: 'ใช้ได้', taken: 'ชื่อผู้ใช้นี้ถูกใช้แล้ว โปรดลองชื่ออื่น', format: 'ใช้ตัวพิมพ์เล็ก ตัวเลข หรือจุด 6–30 ตัว (ห้าม ".." และห้ามจุดที่ต้นหรือท้าย)', reserved: 'ชื่อผู้ใช้นี้ถูกสงวนไว้', suggestions: 'ใช้ได้:',
  passwordTitle: 'สร้างรหัสผ่านที่ปลอดภัย', passwordIntro: 'ใช้อย่างน้อย 12 ตัวอักษร การผสมตัวอักษร ตัวเลข และสัญลักษณ์จะปลอดภัยกว่า',
  recoveryTitle: 'เพิ่มข้อมูลการกู้คืน', recoveryIntro: 'ใช้กู้คืนบัญชีหากคุณลืมรหัสผ่าน', recoveryEmail: 'อีเมลสำรอง (ไม่บังคับ)', recoveryEmailHint: 'ใช้อีเมลจากผู้ให้บริการอื่น เราจะส่งลิงก์ยืนยันไปให้',
  phone: 'หมายเลขโทรศัพท์ (ไม่บังคับ)', country: 'รหัสประเทศ', phoneHint: 'จัดเก็บไว้เพื่อกู้คืนบัญชี ยังไม่รองรับการยืนยันทาง SMS จึงยังไม่ได้รับการยืนยัน', recoveryWarning: 'หากไม่มีอีเมลสำรองที่ยืนยันแล้ว จะรีเซ็ตรหัสผ่านที่ลืมไม่ได้',
  reviewTitle: 'ตรวจสอบบัญชีของคุณ', reviewIntro: 'ตรวจสอบข้อมูลก่อนสร้างบัญชี', name: 'ชื่อ', address: 'ที่อยู่อีเมล', none: 'ไม่ได้ระบุ',
  create: 'สร้างบัญชี', creating: 'กำลังสร้างบัญชี…',
  doneTitle: 'บัญชีของคุณพร้อมแล้ว', doneIntro: 'สร้างกล่องจดหมายแล้วและคุณได้เข้าสู่ระบบแล้ว', doneRecoverySent: 'ส่งลิงก์ยืนยันไปยังอีเมลสำรองแล้ว', doneRecoveryNotSent: 'ยังส่งอีเมลยืนยันไปยังอีเมลสำรองไม่ได้ คุณส่งอีกครั้งได้จากหน้าบัญชี', openMail: 'เปิดกล่องจดหมาย', goHome: 'ไปหน้าหลัก',
  unavailable: 'ขณะนี้ยังสร้างบัญชีอีเมลใหม่ไม่ได้ โปรดลองอีกครั้งภายหลัง',
  loginId: 'อีเมลหรือชื่อผู้ใช้', loginIntro: 'เข้าสู่ระบบด้วยชื่อผู้ใช้หรือที่อยู่อีเมล OpenScanner', forgotIntro: 'หากบัญชีมีอีเมลสำรองที่ยืนยันแล้ว เราจะส่งลิงก์รีเซ็ตไปที่อีเมลนั้น',
  recoveryStatus: 'อีเมลสำรอง', verified: 'ยืนยันแล้ว', unverified: 'ยังไม่ยืนยัน', resendRecovery: 'ส่งอีเมลยืนยันอีกครั้ง', mailAddress: 'ที่อยู่อีเมล',
};
export const signupMessages: Record<Locale, SignupCopy> = { en, ko, th };

// Error codes introduced by hosted-mailbox signup; merged into the main error dictionary.
const errorsEn = { USERNAME_TAKEN: 'That username is taken. Choose another.', USERNAME_RESERVED: 'This username is reserved.', INVALID_USERNAME: 'Check the username format.', AGE_REQUIREMENT: 'You must be at least 14 years old to create an account.', RECOVERY_EMAIL_SAME_DOMAIN: 'Use a recovery email at another provider.', INVALID_PHONE: 'Check the phone number and country code.', MAIL_PROVISIONING_UNAVAILABLE: 'New mailbox accounts cannot be created right now.', MAIL_PROVISIONING_FAILED: 'The mailbox could not be created, so no account was made. Try again.', PROVIDER_DELETION_FAILED: 'The mailbox could not be deleted. Try again later.', INVALID_CREDENTIALS: 'Check your username or email and password.' };
type SignupErrors = { [K in keyof typeof errorsEn]: string };
const errorsKo: SignupErrors = { USERNAME_TAKEN: '이미 사용 중인 아이디입니다. 다른 아이디를 선택하세요.', USERNAME_RESERVED: '예약된 아이디라 사용할 수 없습니다.', INVALID_USERNAME: '아이디 형식을 확인하세요.', AGE_REQUIREMENT: '만 14세 이상만 가입할 수 있습니다.', RECOVERY_EMAIL_SAME_DOMAIN: '복구 이메일은 다른 메일 서비스의 주소를 사용하세요.', INVALID_PHONE: '전화번호와 국가 번호를 확인하세요.', MAIL_PROVISIONING_UNAVAILABLE: '지금은 새 메일 계정을 만들 수 없습니다.', MAIL_PROVISIONING_FAILED: '메일함을 만들지 못해 계정이 생성되지 않았습니다. 다시 시도하세요.', PROVIDER_DELETION_FAILED: '메일함을 삭제하지 못했습니다. 잠시 후 다시 시도하세요.', INVALID_CREDENTIALS: '아이디(또는 이메일)와 비밀번호를 확인하세요.' };
const errorsTh: SignupErrors = { USERNAME_TAKEN: 'ชื่อผู้ใช้นี้ถูกใช้แล้ว โปรดเลือกชื่ออื่น', USERNAME_RESERVED: 'ชื่อผู้ใช้นี้ถูกสงวนไว้', INVALID_USERNAME: 'ตรวจสอบรูปแบบชื่อผู้ใช้', AGE_REQUIREMENT: 'ต้องมีอายุอย่างน้อย 14 ปีจึงจะสร้างบัญชีได้', RECOVERY_EMAIL_SAME_DOMAIN: 'ใช้อีเมลสำรองจากผู้ให้บริการอื่น', INVALID_PHONE: 'ตรวจสอบหมายเลขโทรศัพท์และรหัสประเทศ', MAIL_PROVISIONING_UNAVAILABLE: 'ขณะนี้ยังสร้างบัญชีอีเมลใหม่ไม่ได้', MAIL_PROVISIONING_FAILED: 'สร้างกล่องจดหมายไม่สำเร็จ จึงไม่ได้สร้างบัญชี โปรดลองอีกครั้ง', PROVIDER_DELETION_FAILED: 'ลบกล่องจดหมายไม่สำเร็จ โปรดลองอีกครั้งภายหลัง', INVALID_CREDENTIALS: 'ตรวจสอบชื่อผู้ใช้หรืออีเมลและรหัสผ่าน' };
export const signupErrors: Record<Locale, SignupErrors> = { en: errorsEn, ko: errorsKo, th: errorsTh };
