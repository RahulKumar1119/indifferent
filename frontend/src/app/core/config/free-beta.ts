/**
 * The one answer to "is it free?", shared by the landing FAQ, the Help FAQ and
 * the pricing page so they can never say different things (#255).
 */
export const FREE_BETA_FAQ = {
  q: 'Is Indifferent free?',
  a: 'Yes, Indifferent is free during the beta. No credit card or paid plan is required. Each account has daily and hourly usage limits to help keep the service available for everyone. You can see the current limits on our Pricing page.',
  link: { label: 'See the current limits', path: '/pricing' },
} as const;
