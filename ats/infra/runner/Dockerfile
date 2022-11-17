FROM mcr.microsoft.com/playwright:v1.22.2-focal 
ENV NODE_PATH="/usr/lib/node_modules"
RUN mkdir -p /ats/CURRENT
WORKDIR /ats/CURRENT
ADD ./ats/infra ../infra
COPY ./ats/infra/runner/package.json .
COPY ./ats/infra/runner/yarn.lock .
RUN yarn install --frozen-lockfile
CMD cp /suite/*.spec.ts . && npx playwright test ${PWARGS}
