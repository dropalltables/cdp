package docker

import (
	"fmt"

	"github.com/dropalltables/cdp/internal/detect"
)

// GenerateDockerfile generates a Dockerfile for the detected framework
func GenerateDockerfile(framework *detect.FrameworkInfo) string {
	switch framework.Name {
	case "Next.js":
		return generateNextJSDockerfile(framework)
	case "Astro":
		return generateStaticDockerfile(framework, "dist")
	case "Nuxt":
		return generateNuxtDockerfile(framework)
	case "SvelteKit":
		return generateSvelteKitDockerfile(framework)
	case "Vite", "Create React App":
		return generateStaticDockerfile(framework, framework.PublishDirectory)
	case "Hugo":
		return generateHugoDockerfile(framework)
	case "Go":
		return generateGoDockerfile(framework)
	case "Python":
		return generatePythonDockerfile(framework)
	case "Node.js":
		return generateNodeDockerfile(framework)
	case "Static Site":
		return generatePureStaticDockerfile(framework)
	default:
		return generateGenericDockerfile(framework)
	}
}

func generateNextJSDockerfile(f *detect.FrameworkInfo) string {
	return `FROM node:20-alpine AS base

FROM base AS deps
RUN apk add --no-cache libc6-compat
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM base AS runner
WORKDIR /app
ENV NODE_ENV production
RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs
COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
USER nextjs
EXPOSE 3000
ENV PORT 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3000/ || exit 1
CMD ["node", "server.js"]
`
}

func generateStaticDockerfile(f *detect.FrameworkInfo, outputDir string) string {
	return fmt.Sprintf(`FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/%s /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:80/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
`, outputDir)
}

func generateNuxtDockerfile(f *detect.FrameworkInfo) string {
	return `FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/.output ./
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3000/ || exit 1
CMD ["node", ".output/server/index.mjs"]
`
}

func generateSvelteKitDockerfile(f *detect.FrameworkInfo) string {
	return `FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/build ./build
COPY --from=builder /app/package.json ./
COPY --from=builder /app/node_modules ./node_modules
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3000/ || exit 1
CMD ["node", "build"]
`
}

func generateHugoDockerfile(f *detect.FrameworkInfo) string {
	return `FROM klakegg/hugo:ext-alpine AS builder
WORKDIR /app
COPY . .
RUN hugo --minify

FROM nginx:alpine
COPY --from=builder /app/public /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:80/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
`
}

func generateGoDockerfile(f *detect.FrameworkInfo) string {
	return `FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/app .
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/ || exit 1
CMD ["./app"]
`
}

func generatePythonDockerfile(f *detect.FrameworkInfo) string {
	return `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt* pyproject.toml* ./
RUN pip install --no-cache-dir -r requirements.txt 2>/dev/null || pip install --no-cache-dir .
COPY . .
EXPOSE 8000
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8000/')" || exit 1
CMD ["python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
`
}

func generateNodeDockerfile(f *detect.FrameworkInfo) string {
	// Use shell form for CMD to allow complex start commands
	startCmd := f.StartCommand
	if startCmd == "" {
		startCmd = "npm start"
	}
	return fmt.Sprintf(`FROM node:20-alpine
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci --production
COPY . .
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3000/ || exit 1
CMD %s
`, startCmd)
}

func generatePureStaticDockerfile(f *detect.FrameworkInfo) string {
	return `FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:80/ || exit 1
CMD ["nginx", "-g", "daemon off;"]
`
}

func generateGenericDockerfile(f *detect.FrameworkInfo) string {
	return `FROM node:20-alpine
WORKDIR /app
COPY . .
RUN npm install 2>/dev/null || true
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3000/ || exit 1
CMD ["npm", "start"]
`
}
