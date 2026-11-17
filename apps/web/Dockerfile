# Local development target; no production deployment is claimed.
FROM node:22.23.4-bookworm-slim AS development
WORKDIR /workspace
COPY package.json package-lock.json ./
COPY apps/web/package.json ./apps/web/package.json
RUN npm ci --ignore-scripts --no-audit --no-fund
COPY --chown=node:node apps/web ./apps/web
RUN chown -R node:node /workspace/apps/web
USER node
ENV NEXT_TELEMETRY_DISABLED=1
EXPOSE 3000
CMD ["npm", "run", "dev", "--workspace", "@dinequeue/web"]

FROM development AS build
# Non-secret API origin: rewrites are frozen into the Next.js build.
# Rebuild when changing this value; use the same origin at runtime.
ARG API_INTERNAL_URL=http://api:8080
ENV API_INTERNAL_URL=${API_INTERNAL_URL}
RUN npm run build --workspace @dinequeue/web \
    && mkdir -p /workspace/apps/web/public \
    && rm -rf /workspace/apps/web/.next/cache

FROM node:22.23.4-bookworm-slim AS production-dependencies
WORKDIR /workspace
COPY package.json package-lock.json ./
COPY apps/web/package.json ./apps/web/package.json
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund

FROM node:22.23.4-bookworm-slim AS production
WORKDIR /workspace
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    API_INTERNAL_URL=http://api:8080
COPY --from=production-dependencies /workspace/node_modules ./node_modules
COPY package.json ./
COPY --from=build --chown=node:node /workspace/apps/web/package.json ./apps/web/package.json
COPY --from=build --chown=node:node /workspace/apps/web/next.config.mjs ./apps/web/next.config.mjs
COPY --from=build --chown=node:node /workspace/apps/web/config ./apps/web/config
COPY --from=build --chown=node:node /workspace/apps/web/public ./apps/web/public
COPY --from=build --chown=node:node /workspace/apps/web/.next ./apps/web/.next
WORKDIR /workspace/apps/web
USER node
EXPOSE 3000
CMD ["node", "../../node_modules/next/dist/bin/next", "start", "--hostname", "0.0.0.0"]
