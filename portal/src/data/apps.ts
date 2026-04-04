import type { Category } from '../types';

export const categories: Category[] = [
  {
    id: 'sales',
    title: 'Sales',
    apps: [
      {
        id: 'crm-pro',
        name: 'CRM Pro',
        description: 'Manage client relationships, track leads, and monitor',
        icon: 'chart',
        href: '/apps/crm-pro',
      },
      {
        id: 'pipeline-tracker',
        name: 'Pipeline Tracker',
        description:
          'Visualize sales stages and track deals through the conversion funnel',
        icon: 'funnel',
        href: '/apps/pipeline-tracker',
      },
      {
        id: 'quote-generator',
        name: 'Quote Generator',
        description: 'Create, customize, and send professional sales quotes',
        icon: 'document',
        href: '/apps/quote-generator',
      },
      {
        id: 'vendor-connect',
        name: 'Vendor Connect',
        description:
          'Maintain supplier profiles, ratings, and communication history',
        icon: 'handshake',
        href: '/apps/vendor-connect',
      },
    ],
  },
  {
    id: 'purchase',
    title: 'Purchase',
    apps: [
      {
        id: 'purchase-orders',
        name: 'Purchase Orders',
        description:
          'Generate and manage POs for internal and external procurement',
        icon: 'cart',
        href: '/apps/purchase-orders',
      },
    ],
  },
  {
    id: 'customer-care',
    title: 'Customer Care',
    apps: [
      {
        id: 'support-tickets',
        name: 'Support Tickets',
        description:
          'Log, assign, and resolve customer support requests and inquiries',
        icon: 'chat',
        href: '/apps/support-tickets',
      },
      {
        id: 'feedback-portal',
        name: 'Feedback Portal',
        description:
          'Collect, analyze, and report on customer satisfaction feedback',
        icon: 'star',
        href: '/apps/feedback-portal',
      },
    ],
  },
];
