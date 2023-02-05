FROM mcr.microsoft.com/playwright:v1.26.0-focal 
ENV NODE_PATH="/usr/lib/node_modules"
COPY ./acceptance_tests/infra/playwright/* .
RUN yarn install --frozen-lockfile
# /specs is set by the lab, so we only copy the specs on run
CMD cp /specs/*.spec.ts . && npx playwright test ${PWOPTS}
