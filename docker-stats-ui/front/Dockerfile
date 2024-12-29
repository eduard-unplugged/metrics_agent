# ==========
# build-stage
# ==========
FROM node:18-alpine AS build

WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

# ==========
# prod-stage
# ==========
FROM nginx:alpine

# Удаляем дефолтный конфиг, если хотим свой (опционально)
RUN rm /etc/nginx/conf.d/default.conf

# Можно скопировать свой конфиг (если нужно):
# COPY nginx.conf /etc/nginx/conf.d

# Копируем статические файлы из build-стадии
COPY --from=build /app/build /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]