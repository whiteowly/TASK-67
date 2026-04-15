FROM mcr.microsoft.com/playwright:v1.52.0-noble

WORKDIR /e2e

COPY package.json ./
RUN npm install

COPY playwright.config.ts ./
COPY tests/ ./tests/

RUN mkdir -p screenshots test-results playwright-report

CMD ["npx", "playwright", "test"]
