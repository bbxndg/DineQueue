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
