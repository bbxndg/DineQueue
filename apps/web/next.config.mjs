import { apiOrigin } from './config/api-origin.mjs';

const backendOrigin = apiOrigin(process.env.API_INTERNAL_URL);

const nextConfig = {
  poweredByHeader: false,
  async rewrites() {
    return [
      { source: '/api/health/live', destination: `${backendOrigin}/health/live` },
      { source: '/api/health/ready', destination: `${backendOrigin}/health/ready` },
      { source: '/api/v1/auth/login', destination: `${backendOrigin}/api/v1/auth/login` },
      { source: '/api/v1/auth/me', destination: `${backendOrigin}/api/v1/auth/me` },
      { source: '/api/v1/auth/logout', destination: `${backendOrigin}/api/v1/auth/logout` },
      { source: '/api/v1/queues', destination: `${backendOrigin}/api/v1/queues` },
      { source: '/api/v1/queues/:queueId/entries', destination: `${backendOrigin}/api/v1/queues/:queueId/entries` },
      { source: '/api/v1/me/queue-entries/active', destination: `${backendOrigin}/api/v1/me/queue-entries/active` },
      { source: '/api/v1/me/queue-entries/:entryId', destination: `${backendOrigin}/api/v1/me/queue-entries/:entryId` },
      { source: '/api/v1/host/queues', destination: `${backendOrigin}/api/v1/host/queues` },
      { source: '/api/v1/host/queues/:queueId/entries', destination: `${backendOrigin}/api/v1/host/queues/:queueId/entries` },
      { source: '/api/v1/host/queues/:queueId/current', destination: `${backendOrigin}/api/v1/host/queues/:queueId/current` },
      { source: '/api/v1/host/queues/:queueId/call-next', destination: `${backendOrigin}/api/v1/host/queues/:queueId/call-next` },
      { source: '/api/v1/host/queues/:queueId/entries/:entryId/complete', destination: `${backendOrigin}/api/v1/host/queues/:queueId/entries/:entryId/complete` },
    ];
  },
  async headers() {
    return [{ source: '/:path*', headers: [
      { key: 'X-Content-Type-Options', value: 'nosniff' },
      { key: 'X-Frame-Options', value: 'DENY' },
      { key: 'Referrer-Policy', value: 'same-origin' },
    ] }];
  },
};

export default nextConfig;
