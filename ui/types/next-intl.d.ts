import en from '../messages/en.json';

type Messages = typeof en;

// Use type-safe message keys with next-intl
declare global {
  interface IntlMessages extends Messages {}
}
