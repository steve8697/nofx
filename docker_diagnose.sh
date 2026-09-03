#!/bin/bash

# ═══════════════════════════════════════════════════════════════
# AETHERIS AI Trading System - Docker 一鍵深度診斷與自動修復指令碼
# ═══════════════════════════════════════════════════════════════

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

print_header() {
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}             🐳 AETHERIS Docker 深度診斷與一鍵修復工具${NC}"
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
}

print_info() { echo -e "${BLUE}[資訊]${NC} $1"; }
print_success() { echo -e "${GREEN}[成功]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[警告]${NC} $1"; }
print_error() { echo -e "${RED}[錯誤]${NC} $1"; }

# 1. 檢查 Docker 狀態與自動啟動
check_docker_daemon() {
    print_header
    print_info "正在檢測 Docker Daemon 狀態..."
    
    if ! command -v docker &> /dev/null; then
        print_error "未檢測到 Docker CLI 工具！請先安裝 Docker Desktop。"
        exit 1
    fi
    
    # 嘗試連接 Docker Daemon
    if ! docker info &> /dev/null; then
        print_warning "Docker Daemon 未啟動或無法連線！"
        print_info "正在嘗試為您啟動 macOS Docker Desktop..."
        
        # 在 Mac 上自動開啟 Docker Desktop
        if [ -d "/Applications/Docker.app" ]; then
            open -a Docker
            print_info "已發送啟動指令給 Docker Desktop，正在等待 Docker 啟動 (最多等待 30 秒)..."
            
            for i in {1..30}; do
                if docker info &> /dev/null; then
                    print_success "Docker Daemon 已成功啟動並連線！"
                    return 0
                fi
                echo -n "."
                sleep 2
            done
            echo ""
            print_error "Docker 啟動逾時。請確認您已在 Mac 手動開啟 Docker Desktop 並授權。"
            exit 1
        else
            print_error "未在 /Applications/Docker.app 找到 Docker Desktop，請手動開啟 Docker。"
            exit 1
        fi
    else
        print_success "Docker Daemon 運行正常且連線成功！"
    fi
}

# 2. 檢測 config.db 是否被 Docker 誤創為資料夾
check_database_file() {
    print_info "正在檢測資料庫檔案狀態..."
    if [ -d "config.db" ]; then
        print_error "偵測到嚴重 Bug：config.db 被 Docker 誤創建為【資料夾】！"
        print_info "這會導致 SQLite 無法讀寫，後端容器會崩潰。"
        print_info "正在自動修復資料庫檔案..."
        rm -rf config.db
        touch config.db
        print_success "已成功刪除異常目錄並重新建立了空 config.db 檔案！"
    elif [ -f "config.db" ]; then
        print_success "config.db 檔案狀態正常。"
    else
        print_warning "config.db 不存在，正在為您創建空檔案以防掛載出錯..."
        touch config.db
        print_success "空 config.db 建立完成。"
    fi
}

# 3. 檢查埠占用
check_ports() {
    print_info "正在檢測系統埠占用情況..."
    
    # 檢測 3636 後端埠
    local backend_pid=$(lsof -t -i:3636 2>/dev/null || true)
    if [ ! -z "$backend_pid" ]; then
        print_warning "埠 3636 已被本地行程 (PID: $backend_pid) 占用！"
        # 排除是 Docker 容器本身的占用
        local docker_using=$(docker ps --filter "publish=3636" --format "{{.Names}}" || true)
        if [ ! -z "$docker_using" ]; then
            print_info "該埠正由 Docker 容器 '$docker_using' 正常使用中。"
        else
            print_warning "非 Docker 行程佔用了 3636 埠。正在自動釋放 3636 埠..."
            kill -9 $backend_pid &>/dev/null || true
            print_success "已強制釋放 3636 埠。"
        fi
    else
        print_success "埠 3636 處於空閒狀態。"
    fi
    
    # 檢測 3434 前端埠
    local frontend_pid=$(lsof -t -i:3434 2>/dev/null || true)
    if [ ! -z "$frontend_pid" ]; then
        print_warning "埠 3434 已被本地行程 (PID: $frontend_pid) 占用！"
        local docker_using_fe=$(docker ps --filter "publish=3434" --format "{{.Names}}" || true)
        if [ ! -z "$docker_using_fe" ]; then
            print_info "該埠正由 Docker 容器 '$docker_using_fe' 正常使用中。"
        else
            print_warning "非 Docker 行程佔用了 3434 埠。正在自動釋放 3434 埠..."
            kill -9 $frontend_pid &>/dev/null || true
            print_success "已強制釋放 3434 埠。"
        fi
    else
        print_success "埠 3434 處於空閒狀態。"
    fi
}

# 4. 檢查環境變數與配置
check_env_configs() {
    print_info "正在核對環境變數與安全配置..."
    
    if [ ! -f ".env" ]; then
        print_warning ".env 檔案不存在，自動從範本複製..."
        cp .env.example .env
        print_success "已生成預設 .env 檔案。"
    fi
    
    if [ ! -f "config.json" ]; then
        print_warning "config.json 不存在，自動從範本複製..."
        cp config.json.example config.json
        print_success "已生成預設 config.json 檔案。"
    fi
    
    # 讀取 admin_mode
    local admin_mode=$(grep -o '"admin_mode":\s*[a-z]*' config.json | head -1 | cut -d':' -f2 | tr -d ' ' || echo "false")
    # 讀取 .env 中的密碼
    local admin_pass=$(grep "^AETHERIS_ADMIN_PASSWORD=" .env | cut -d'=' -f2- | tr -d ' ' | tr -d '"' | tr -d "'" || echo "")
    
    if [ "$admin_mode" = "true" ] && [ -z "$admin_pass" ]; then
        print_error "安全配置 Bug：已啟用管理員模式，但 .env 中未設定 AETHERIS_ADMIN_PASSWORD！"
        print_info "正在自動為您設定預設密碼 'admin123' 以防後端崩潰..."
        # 寫入密碼
        if grep -q "^AETHERIS_ADMIN_PASSWORD=" .env; then
            sed -i '' 's/^AETHERIS_ADMIN_PASSWORD=.*/AETHERIS_ADMIN_PASSWORD=admin123/' .env
        else
            echo "AETHERIS_ADMIN_PASSWORD=admin123" >> .env
        fi
        print_success "已將預設密碼 'admin123' 寫入 .env！"
    else
        print_success "管理員帳號與密碼安全配置檢查通過。"
    fi
}

# 5. 清理並重新部署 Docker 服務
rebuild_and_restart() {
    echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
    print_info "正在執行一鍵自動化重構與重啟服務..."
    
    # 停止現有容器並清除可能殘存的掛載卷快取
    print_info "正在清理舊的 Docker 容器與 volumes..."
    docker compose down -v 2>/dev/null || docker-compose down -v 2>/dev/null || true
    
    # 重新構建並啟動服務
    print_info "正在重新構建前端與後端鏡像並啟動服務 (這可能需要數分鐘，請稍候)..."
    
    local compose_cmd="docker compose"
    if ! command -v docker compose &> /dev/null; then
        compose_cmd="docker-compose"
    fi
    
    if $compose_cmd up -d --build; then
        echo -e "${PURPLE}═══════════════════════════════════════════════════════════════${NC}"
        print_success "🎉 所有修復與重啟操作已圓滿完成！"
        
        # 讀取對外顯示端口
        local fe_port=$(grep "^AETHERIS_FRONTEND_PORT=" .env | cut -d'=' -f2 | tr -d ' ' || echo "3434")
        local be_port=$(grep "^AETHERIS_BACKEND_PORT=" .env | cut -d'=' -f2 | tr -d ' ' || echo "3636")
        
        print_info "WEB UI 界面位址: http://localhost:${fe_port}"
        print_info "後端 API 健康指標: http://localhost:${be_port}/api/health"
        print_info "如果您使用的是 Safari 或是 Chrome，請重新整理網頁即可正常開啟！"
    else
        print_error "Docker 服務重新構建或啟動失敗。請查看日誌：docker compose logs"
    fi
}

# 執行主控流程
main() {
    check_docker_daemon
    check_database_file
    check_ports
    check_env_configs
    rebuild_and_restart
}

main "$@"
