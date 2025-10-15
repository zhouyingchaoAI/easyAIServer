SHELL := /bin/bash

.PHONY: build/linux build/local clean frontend backend fx-enable fx-add fx-check-ffmpeg fx-run-example

# Frontend build (requires Node.js >= 18)
frontend:
	cd web-src && \
	if [ -f package-lock.json ]; then \
		npm ci; \
	else \
		npm install; \
	fi && \
	npm run build && \
	if [ -d web ]; then \
		rm -rf ../web/* && cp -r web/* ../web/; \
	elif [ -d dist ]; then \
		rm -rf ../web/* && cp -r dist/* ../web/; \
	else \
		echo "Error: No build output found (checked web/ and dist/)" && exit 1; \
	fi

# Backend build for current OS/ARCH
backend:
	go build -o build/easydarwin ./cmd/server

# Cross-compile for Linux amd64 and package web assets
build/linux: frontend
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
	go build -o build/easydarwin_linux_amd64 ./cmd/server
	# bundle ffmpeg if exists in repo root (optional)
	@if [ -f ffmpeg ]; then cp ffmpeg build/; fi
	@echo "Build artifacts: build/easydarwin_linux_amd64 and web/ assets"

# Local build (frontend + backend for host platform)
build/local: frontend backend
	@echo "Build artifacts: build/easydarwin and web/ assets"

clean:
	rm -rf build/* web/*

# --- Frame Extractor helpers ---
# Enable frame extractor module in configs/config.toml
fx-enable:
	sed -i 's/^enable\s*=\s*false/enable = true/' configs/config.toml
	@echo "frame_extractor.enable set to true"

# --- AI Analysis helpers ---
# Enable AI analysis module in configs/config.toml
ai-enable:
	sed -i '/\[ai_analysis\]/,/^$$/ s/^enable\s*=\s*false/enable = true/' configs/config.toml
	@echo "ai_analysis.enable set to true"

# Start example algorithm service
# Usage: make ai-start-algo [PORT=8000 TYPES="人数统计"]
ai-start-algo:
	@PORT=$${PORT:-8000}; \
	TYPES=$${TYPES:-人数统计}; \
	echo "Starting algorithm service on port $$PORT..."; \
	python3 examples/algorithm_service.py \
	  --service-id demo_algo_$$PORT \
	  --name "演示算法服务" \
	  --task-types $$TYPES \
	  --port $$PORT \
	  --easydarwin http://localhost:5066

# Test AI analysis with algorithm service
# Usage: make ai-test
ai-test:
	@echo "Testing AI Analysis..."
	@echo "1. Checking algorithm services..."
	@curl -s http://localhost:5066/api/v1/ai_analysis/services | jq '.total // 0'
	@echo "2. Checking alerts..."
	@curl -s http://localhost:5066/api/v1/alerts | jq '.total // 0'
	@echo "3. Access UI:"
	@echo "   Alerts: http://localhost:5066/#/alerts"
	@echo "   Services: http://localhost:5066/#/ai-services"

# Add a frame extractor task via API
# Usage: make fx-add ID=cam1 RTSP=rtsp://user:pass@ip:554/... INTERVAL=1000 OUT=cam1 SERVER=127.0.0.1:10086
fx-add:
	@if [ -z "$(ID)" ] || [ -z "$(RTSP)" ]; then \
		echo "Usage: make fx-add ID=cam1 RTSP=rtsp://... [INTERVAL=1000 OUT=cam1 SERVER=127.0.0.1:10086]"; \
		exit 1; \
	fi
	@SERVER_ADDR=$${SERVER:-127.0.0.1:10086}; \
	INTERVAL_MS=$${INTERVAL:-1000}; \
	OUT_PATH=$${OUT:-$(ID)}; \
	curl -sS -X POST http://$$SERVER_ADDR/api/v1/frame_extractor/tasks \
	  -H 'Content-Type: application/json' \
	  -d '{"id":"$(ID)","rtsp_url":"$(RTSP)","interval_ms":'"$$INTERVAL_MS"',"output_path":"'"$$OUT_PATH"'"}' && echo

# Check ffmpeg availability
fx-check-ffmpeg:
	@which ffmpeg >/dev/null 2>&1 && echo "ffmpeg found in PATH" || \
	([ -f ./ffmpeg ] && echo "./ffmpeg present" || (echo "ffmpeg not found. Place binary at ./ffmpeg or install system ffmpeg" && exit 1))

# Run server (current platform), enable plugin, then add a sample task
# Usage: make fx-run-example RTSP=rtsp://user:pass@ip:554/... [ID=cam1 INTERVAL=1000 OUT=cam1]
fx-run-example: build/local fx-enable fx-check-ffmpeg
	@echo "Starting server..."
	@( ./build/easydarwin -conf ./configs & echo $$! > build/server.pid )
	@sleep 2
	$(MAKE) fx-add ID=$${ID:-cam1} RTSP="$(RTSP)" INTERVAL=$${INTERVAL:-1000} OUT=$${OUT:-$${ID:-cam1}} SERVER=127.0.0.1:10086
	@echo "Visit http://127.0.0.1:10086/#/frame-extractor to manage tasks"
# Makefile 使用文档
# https://www.gnu.org/software/make/manual/html_node/index.html

# include .envrc
SHELL = /bin/bash

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/n] ' && read ans && [ $${ans:-N} = y ]

.PHONY: title
title:
	@echo -e "\033[34m$(content)\033[0m"

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## init: 安装开发环境
init:
	go install github.com/google/wire/cmd/wire@latest
	go install github.com/divan/expvarmon@latest
	go install github.com/rakyll/hey@latest
	go install mvdan.cc/gofumpt@latest

## wire: 生成依赖注入代码
wire:
	go mod tidy
	go get github.com/google/wire/cmd/wire@latest
	go generate ./...
	go mod tidy

## expva/http: 监听网络请求指标
expva/http:
	expvarmon --ports=":9999" -i 1s -vars="version,request,requests,responses,goroutines,errors,panics,mem:memstats.Alloc"

## expva/db: 监听数据库连接指标
expva/db:
	expvarmon --ports=":9999" -i 5s -vars="databse.MaxOpenConnections,databse.OpenConnections,database.InUse,databse.Idle"

# 发起 100 次请求，每次并发 50
# hey -n 100 -c 50 http://localhost:9999/healthcheck


# ==================================================================================== #
# VERSION
# ==================================================================================== #

# 版本号规则说明
# 1. 版本号使用 Git tag，格式为 v1.0.0。
# 2. 如果当前提交没有 tag，找到最近的 tag，计算从该 tag 到当前提交的提交次数。例如，最近的 tag 为 v1.0.1，当前提交距离它有 10 次提交，则版本号为 v1.0.11（v1.0.1 + 10 次提交）。
# 3. 如果没有任何 tag，则默认版本号为 v0.0.0，后续提交次数作为版本号的次版本号。

# Get the current module name
MODULE_NAME := $(shell pwd | awk -F "/" '{print $$NF}')
# Get the latest commit hash and date
HASH_AND_DATE := $(shell git log -n1 --pretty=format:"%h-%cd" --date=format:%y%m%d | awk '{print $1}')
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

# 如果想仅支持注释标签，可以去掉 --tags，否则会包含轻量标签
RECENT_TAG := $(shell git describe --tags --abbrev=0  2>&1 | grep -v "fatal" || echo "v0.0.0")

ifeq ($(RECENT_TAG),v0.0.0)
	COMMITS := $(shell git rev-list --count HEAD)
else
	COMMITS := $(shell git log --first-parent --format='%ae' $(RECENT_TAG)..$(BRANCH) | wc -l)
	COMMITS := $(shell echo $(COMMITS) | sed 's/ //g')
endif

# 从版本字符串中提取主版本号、次版本号和修订号
GIT_VERSION_MAJOR := $(shell echo $(RECENT_TAG) | cut -d. -f1 | sed 's/v//')
GIT_VERSION_MINOR := $(shell echo $(RECENT_TAG) | cut -d. -f2)
GIT_VERSION_PATCH := $(shell echo $(RECENT_TAG) | cut -d. -f3)

# windows 系统 git bash 没有 bc
# FINAL_PATCH := $(shell echo $(GIT_VERSION_PATCH) + $(COMMITS) | bc)
FINAL_PATCH := $(shell echo '$(GIT_VERSION_PATCH) $(COMMITS)' | awk '{print $$1 + $$2}')
#VERSION := v$(GIT_VERSION_MAJOR).$(GIT_VERSION_MINOR).$(FINAL_PATCH)
VERSION := v8.3.3
# test:
# 	@echo ">>>${RECENT_TAG}"

## info: 查看构建版本相关信息
.PHONY: info
info:
	@echo "dir: $(MODULE_NAME)"
	@echo "version: $(VERSION)"
	@echo "branch $(BRANCH)"
	@echo "hash: $(HASH_AND_DATE)"
	@echo "support $$(go tool dist list | grep amd64 | grep linux)"


# ==================================================================================== #
# BUILD
# ==================================================================================== #

BUILD_DIR_ROOT := ./build
GOOS = $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)
IMAGE_NAME := $(MODULE_NAME):latest
DAY_TIME:=$(shell date +"%Y%m%d%H%M")
## build/clean: 清理构建缓存目录
.PHONY: build/clean
build/clean:
	@rm -rf $(BUILD_DIR_ROOT)/*

## build/local: 构建本地应用
.PHONY: build/local
build/local:
	$(eval NAME := EasyDarwin-lin)
	$(eval dir := $(BUILD_DIR_ROOT)/$(NAME)-$(VERSION)-$(DAY_TIME))
	@echo 'Building $(VERSION) $(dir)...'
	@rm -rf $(dir)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-trimpath \
		-ldflags="-s -w \
			-X main.buildVersion=$(VERSION) \
			-X main.gitHash=$(HASH_AND_DATE) \
			-X main.buildTimeAt=$(shell date +%s) \
			-X main.release=true \
			" -o=$(dir)/easydarwin.com ./cmd/server
	@scp -r web configs ${BUILD_LINUX_AMD64_DIR}
	@scp deploy/*.sh deploy/easydarwin ${BUILD_LINUX_AMD64_DIR}
	@scp deploy/ffmpeg ${BUILD_LINUX_AMD64_DIR}
	@rm -rf ${BUILD_LINUX_AMD64_DIR}/configs/data.db
	@echo '>>> OK'

## build/linux: 构建 linux 应用
.PHONY: build/linux
BUILD_LINUX_AMD64_DIR := ./build/EasyDarwin-lin-$(VERSION)-$(DAY_TIME)
build/linux:
	$(eval GOARCH := amd64)
	$(eval GOOS := linux)
	@make build/local GOOS=$(GOOS) GOARCH=$(GOARCH)

## build/windows: 构建 windows 应用
.PHONY: build/windows
BUILD_WINDOWS_AMD64_DIR := ./build/EasyDarwin-win-$(VERSION)-$(DAY_TIME)
build/windows:
	$(eval GOARCH := amd64)
	$(eval GOOS := windows)
	$(eval NAME := EasyDarwin-win)
	$(eval dir := $(BUILD_DIR_ROOT)/$(NAME)-$(VERSION)-$(DAY_TIME))
	@echo 'Building $(VERSION) $(dir)...'
	@rm -rf $(dir)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-trimpath \
		-ldflags="-s -w \
			-X main.buildVersion=$(VERSION) \
			-X main.gitHash=$(HASH_AND_DATE) \
			-X main.buildTimeAt=$(shell date +%s) \
			-X main.release=true \
			" -o=$(dir)/EasyDarwin.exe ./cmd/server
	@scp -r web configs ${dir}
	@scp ffmpeg.exe ${dir}
	@rm -rf ${dir}/configs/data.db
	@echo '>>> OK'

## build/arm: 构建Arm应用
.PHONY: build/arm
build/arm:
	$(eval NAME := EasyDarwin-aarch64)
	$(eval dir := $(BUILD_DIR_ROOT)/$(NAME)-$(VERSION)-$(DAY_TIME))
	@echo 'Building $(VERSION) $(dir)...'
	@rm -rf $(dir)
	@CGO_ENABLED=$(CGO_ENABLED) CC=$(CC) GOARM=$(GOARM) GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-trimpath \
		-ldflags="-s -w \
			-X main.buildVersion=$(VERSION) \
			-X main.gitHash=$(HASH_AND_DATE) \
			-X main.buildTimeAt=$(shell date +%s) \
			-X main.release=true \
			" -o=$(dir)/easydarwin.com ./cmd/server
	@scp -r web configs ${BUILD_LINUX_ARM64_DIR}
	@scp deploy/*.sh deploy/arm/easydarwin ${BUILD_LINUX_ARM64_DIR}
	@scp deploy/arm/ffmpeg ${BUILD_LINUX_ARM64_DIR}
	@rm -rf ${BUILD_LINUX_ARM64_DIR}/configs/data.db
	@echo '>>> OK'

## build/arm64: 构建 arm64 应用
.PHONY: build/arm64
BUILD_LINUX_ARM64_DIR := ./build/EasyDarwin-aarch64-$(VERSION)-$(DAY_TIME)
build/arm64:
	$(eval CGO_ENABLED := 1)
	$(eval CC := aarch64-linux-gnu-gcc)
	$(eval GOARM := 7)
	$(eval GOARCH := arm64)
	$(eval GOOS := linux)
	@make build/arm CGO_ENABLED=$(CGO_ENABLED) CC=$(CC) GOARM=$(GOARM) GOOS=$(GOOS) GOARCH=$(GOARCH)

docker/build:
	@docker buildx build --force-rm=true --platform linux/amd64 -t $(IMAGE_NAME) .

docker/save:
	@docker save -o $(MODULE_NAME)_$(VERSION).tar $(IMAGE_NAME)

docker/push:
	@docker push $(IMAGE_NAME)

# ==================================================================================== #
# PRODUCTION
# ==================================================================================== #

PRODUCTION_HOST = remoteHost

## release/push: 发布产品到服务器，仅上传文件
# 中小项目可以引入 CI/CD，也可以通过命令快速发布到测试服务器上。
release/push:
	@scp build/linux_amd64/bin $(PRODUCTION_HOST):/home/app/$(MODULE_NAME)
	@echo "push Successed"
