FROM node:20-alpine AS builder
WORKDIR /app

# Enable Next.js telemetry collection off (optional but good practice for CI)
ENV NEXT_TELEMETRY_DISABLED 1

# Install dependencies first (caching layer)
COPY package.json package-lock.json* ./
RUN npm install

# Copy source code and build
COPY . .
RUN npm run build

# Production runner image
FROM node:20-alpine AS runner
WORKDIR /app

ENV NODE_ENV production
ENV NEXT_TELEMETRY_DISABLED 1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

# Copy built assets
# COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE $PORT
ENV PORT=$PORT

CMD ["node", "server.js"]
