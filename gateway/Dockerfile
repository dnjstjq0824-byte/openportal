# Production image for the Node web gateway (B-1).
# Build context is the REPO ROOT: docker build -f gateway/Dockerfile .
FROM node:24.19.0-bookworm-slim AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts

FROM node:24.19.0-bookworm-slim
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY package.json ./package.json
COPY gateway ./gateway
COPY scripts/migrate.mjs ./scripts/migrate.mjs
COPY database ./database
# Non-root execution (node user, uid 1000).
USER node
ENV NODE_ENV=production GATEWAY_HOST=0.0.0.0
EXPOSE 8090
HEALTHCHECK --interval=10s --timeout=5s --retries=12 --start-period=20s \
  CMD node --input-type=module -e "fetch('http://127.0.0.1:8090/api/v2/session').then(r=>{process.exit(r.status===200||r.status===401?0:1)}).catch(()=>process.exit(1))"
CMD ["node", "gateway/server.mjs"]
