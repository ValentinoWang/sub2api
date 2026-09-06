FROM mcr.microsoft.com/playwright:v1.62.1-noble

ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --include=dev

COPY tsconfig.json ./
COPY src ./src
RUN npm run build && npm prune --omit=dev

ENV NODE_ENV=production

USER pwuser

CMD ["node", "dist/index.js"]
