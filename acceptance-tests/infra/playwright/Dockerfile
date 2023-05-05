FROM mcr.microsoft.com/playwright:v1.26.0-focal 
ENV NODE_PATH="/usr/lib/node_modules"
RUN mkdir /runner
WORKDIR /runner
COPY ./acceptance-tests/infra/playwright/* .
RUN yarn install --frozen-lockfile
# /specs is set by lab.yaml, here copy the specs on run
# to make sure we're running the latest version
CMD cp /specs/*.spec.ts . && npx playwright test ${PWOPTS}
