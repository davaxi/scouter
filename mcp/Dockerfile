# syntax=docker/dockerfile:1.7
#
# MCP server (thin adapter over /api/v1). Published as davaxi/scouter-mcp.
# Stage 1 assembles /app (prod deps + sources) so the runtime image gets it in
# a single layer.
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm install --omit=dev --no-audit --no-fund
COPY server.js tools.js seo-playbook.js ./

FROM node:22-alpine
ENV NODE_ENV=production
WORKDIR /app
COPY --from=build /app /app
USER node
EXPOSE 3000
CMD ["node", "server.js"]
