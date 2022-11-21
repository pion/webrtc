FROM mcr.microsoft.com/playwright:v1.26.0-focal 
ENV NODE_PATH="/usr/lib/node_modules"
RUN mkdir -p /ats/CUT
WORKDIR /ats/CUT
ADD ./ats/infra ../infra
COPY ./ats/infra/playwright/* .
RUN yarn install --frozen-lockfile
CMD cp /specs/*.spec.ts . && npx playwright test ${PWOPTS}
