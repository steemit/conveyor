FROM node:8-alpine

RUN apk add --no-cache make bash

WORKDIR /app
COPY . .

RUN make ci-test
RUN make lib

# reinstall modules for production
RUN rm -r node_modules && npm install --production

EXPOSE 8080

ENV PORT 8080
ENV NODE_ENV production

CMD [ "node", "lib/server.js" ]
