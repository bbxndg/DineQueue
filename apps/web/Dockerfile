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
