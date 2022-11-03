FROM mcr.microsoft.com/playwright:v1.22.2-focal 
ENV NODE_PATH="/usr/lib/node_modules"
RUN mkdir -p /aatp/CURRENT
WORKDIR /aatp/CURRENT
ADD ./aatp/infra ../infra
COPY ./aatp/infra/runner/package.json .
COPY ./aatp/infra/runner/yarn.lock .
RUN yarn install --frozen-lockfile
CMD cp /suite/*.spec.ts . && npx playwright test ${PWARGS}
