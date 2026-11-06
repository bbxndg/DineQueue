import { apiOrigin } from './config/api-origin.mjs';

const backendOrigin = apiOrigin(process.env.API_INTERNAL_URL);

const nextConfig = {
  poweredByHeader: false,
  async rewrites() {
    return [
      { source: '/api/health/live', destination: `${backendOrigin}/health/live` },
      { source: '/api/health/ready', destination: `${backendOrigin}/health/ready` },
    ];
  },
};

export default nextConfig;
