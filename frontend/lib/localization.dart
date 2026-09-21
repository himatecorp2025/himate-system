part of 'main.dart';

const himateSupportedLocales = <Locale>[
  Locale('en', 'US'),
  Locale('hu', 'HU'),
];

Locale himateLocaleFromCode(String? code) {
  switch ((code ?? '').trim()) {
    case 'hu_HU':
      return const Locale('hu', 'HU');
    default:
      return const Locale('en', 'US');
  }
}

String himateLocaleCode(Locale locale) =>
    locale.languageCode.toLowerCase() == 'hu' ? 'hu_HU' : 'en_US';

String tr(BuildContext context, String key) =>
    HimateI18n.text(himateLocaleCode(Localizations.localeOf(context)), key);

class HimateI18n {
  static const Map<String, Map<String, String>> _values = {
    'en_US': {
      'account': 'Account',
      'profile': 'Profile',
      'signOut': 'Sign out',
      'language': 'Language',
      'englishUS': 'English (US)',
      'hungarian': 'Magyar',
      'fullName': 'Full name',
      'jobTitle': 'Job title',
      'phone': 'Phone',
      'timezone': 'Time zone',
      'saveChanges': 'Save changes',
      'profileUpdated': 'Profile updated',
      'profileIntro': 'Personal settings for your HIMATE account.',
      'systemOwner': 'System Owner',
      'systemOwnerHint': 'This account can create users and manage access.',
      'roles': 'Roles',
      'passwordSecurity': 'Password & security',
      'currentPassword': 'Current password',
      'newPassword': 'New password',
      'confirmPassword': 'Confirm new password',
      'changePassword': 'Change password',
      'passwordChanged': 'Password changed. Other sessions were invalidated.',
      'passwordsMismatch': 'The new passwords do not match.',
      'passwordMin': 'Use at least 12 characters.',
      'cancel': 'Cancel',
      'close': 'Close',
      'welcomeBack': 'Welcome back',
      'signInSubtitle': 'Sign in to your HIMATE System account',
      'emailAddress': 'Email address',
      'password': 'Password',
      'rememberMe': 'Remember me',
      'forgotPassword': 'Forgot password?',
      'signIn': 'Sign in',
      'showPassword': 'Show password',
      'hidePassword': 'Hide password',
      'enterCredentials': 'Enter your administrator email and password.',
      'recoveryPending': 'Password recovery will be connected in the security phase.',
      'ssoPending': 'SSO is not configured for this environment yet.',
      'nav.dashboard': 'Dashboard',
      'nav.dashboardSub': 'Platform overview',
      'nav.partners': 'Partners',
      'nav.partnersSub': 'Partner control',
      'nav.finance': 'Licensing & Finance',
      'nav.financeSub': 'Commercial management',
      'nav.impact': 'Impact & Reports',
      'nav.impactSub': 'Metrics and reporting',
      'nav.website': 'Website & Marketing',
      'nav.websiteSub': 'Brand and growth',
      'nav.system': 'System & Operations',
      'nav.systemSub': 'Infrastructure health',
      'nav.admin': 'Administration',
      'nav.adminSub': 'Roles and control',
    },
    'hu_HU': {
      'account': 'Fiók',
      'profile': 'Profil',
      'signOut': 'Kijelentkezés',
      'language': 'Nyelv',
      'englishUS': 'Angol (USA)',
      'hungarian': 'Magyar',
      'fullName': 'Teljes név',
      'jobTitle': 'Munkakör',
      'phone': 'Telefonszám',
      'timezone': 'Időzóna',
      'saveChanges': 'Módosítások mentése',
      'profileUpdated': 'Profil frissítve',
      'profileIntro': 'A HIMATE-fiók személyes beállításai.',
      'systemOwner': 'Rendszertulajdonos',
      'systemOwnerHint': 'Ez a fiók hozhat létre felhasználókat és kezelheti a hozzáféréseket.',
      'roles': 'Szerepkörök',
      'passwordSecurity': 'Jelszó és biztonság',
      'currentPassword': 'Jelenlegi jelszó',
      'newPassword': 'Új jelszó',
      'confirmPassword': 'Új jelszó megerősítése',
      'changePassword': 'Jelszó módosítása',
      'passwordChanged': 'A jelszó megváltozott. A többi munkamenet érvénytelenítve lett.',
      'passwordsMismatch': 'Az új jelszavak nem egyeznek.',
      'passwordMin': 'Legalább 12 karakter szükséges.',
      'cancel': 'Mégse',
      'close': 'Bezárás',
      'welcomeBack': 'Üdv újra',
      'signInSubtitle': 'Jelentkezz be a HIMATE System fiókodba',
      'emailAddress': 'E-mail-cím',
      'password': 'Jelszó',
      'rememberMe': 'Maradjak bejelentkezve',
      'forgotPassword': 'Elfelejtetted a jelszót?',
      'signIn': 'Bejelentkezés',
      'showPassword': 'Jelszó megjelenítése',
      'hidePassword': 'Jelszó elrejtése',
      'enterCredentials': 'Add meg az adminisztrátori e-mail-címedet és jelszavadat.',
      'recoveryPending': 'A jelszó-helyreállítás a biztonsági fázisban kerül bekötésre.',
      'ssoPending': 'Ebben a környezetben az SSO még nincs konfigurálva.',
      'nav.dashboard': 'Irányítópult',
      'nav.dashboardSub': 'Platform áttekintése',
      'nav.partners': 'Partnerek',
      'nav.partnersSub': 'Partnerkezelés',
      'nav.finance': 'Licencelés és pénzügy',
      'nav.financeSub': 'Kereskedelmi kezelés',
      'nav.impact': 'Hatás és jelentések',
      'nav.impactSub': 'Mérőszámok és riportok',
      'nav.website': 'Weboldal és marketing',
      'nav.websiteSub': 'Márka és növekedés',
      'nav.system': 'Rendszer és üzemeltetés',
      'nav.systemSub': 'Infrastruktúra állapota',
      'nav.admin': 'Adminisztráció',
      'nav.adminSub': 'Szerepkörök és kontroll',
    },
  };

  static String text(String locale, String key) {
    final normalized = locale == 'hu_HU' ? 'hu_HU' : 'en_US';
    return _values[normalized]?[key] ?? _values['en_US']?[key] ?? key;
  }

  static String dateTime(String locale, DateTime value) {
    final tag = locale == 'hu_HU' ? 'hu_HU' : 'en_US';
    return intl.DateFormat.yMMMd(tag).add_Hm().format(value.toLocal());
  }

  static String currency(String locale, num value, {String currency = 'USD'}) {
    final tag = locale == 'hu_HU' ? 'hu_HU' : 'en_US';
    return intl.NumberFormat.simpleCurrency(locale: tag, name: currency).format(value);
  }
}
