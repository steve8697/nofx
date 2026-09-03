#!/bin/bash

# NoFX Trading Bot - PM2 管理脚本
# 用法: ./pm2.sh [start|stop|restart|status|logs|build]

set -e

# 自动获取脚本所在目录（支持符号链接）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_ROOT"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 函数：打印带颜色的消息
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_header() {
    echo -e "${PURPLE}═══════════════════════════════════════${NC}"
    echo -e "${PURPLE}  🤖 NoFX Trading Bot - PM2 Manager${NC}"
    echo -e "${PURPLE}═══════════════════════════════════════${NC}"
    echo ""
}

# 函数：检查 PM2 是否安装
check_pm2() {
    if ! command -v pm2 &> /dev/null; then
        print_error "PM2 未安装，请先安装: npm install -g pm2"
        exit 1
    fi
}

# 函数：确保日志目录存在
ensure_log_dirs() {
    mkdir -p "$PROJECT_ROOT/logs"
    mkdir -p "$PROJECT_ROOT/web/logs"
    print_info "日志目录已创建"
}

# 函数：编译后端
build_backend() {
    print_info "正在编译后端..."
    go build -o aetheris
    if [ $? -eq 0 ]; then
        print_success "后端编译完成"
    else
        print_error "后端编译失败"
        exit 1
    fi
}

# 函数：构建前端（生产环境）
build_frontend() {
    print_info "正在构建前端..."
    cd web
    npm run build
    if [ $? -eq 0 ]; then
        print_success "前端构建完成"
        cd ..
    else
        print_error "前端构建失败"
        exit 1
    fi
}

# 函数：启动服务
start_services() {
    print_header
    ensure_log_dirs

    # 检查后端二进制文件是否存在
    if [ ! -f "./aetheris" ]; then
        print_warning "后端二进制文件不存在，开始编译..."
        build_backend
    fi

    print_info "正在启动服务..."
    pm2 start pm2.config.js

    sleep 2
    pm2 status

    echo ""
    print_success "服务启动完成！"
    echo ""
    echo -e "${CYAN}📊 访问地址:${NC}"
    echo -e "  ${GREEN}前端:${NC} http://localhost:3434"
    echo -e "  ${GREEN}后端 API:${NC} http://localhost:3636"
    echo ""
    echo -e "${CYAN}📝 查看日志:${NC}"
    echo -e "  ${GREEN}实时日志:${NC} ./pm2.sh logs"
    echo -e "  ${GREEN}后端日志:${NC} ./pm2.sh logs backend"
    echo -e "  ${GREEN}前端日志:${NC} ./pm2.sh logs frontend"
    echo ""
}

# 函数：停止服务
stop_services() {
    print_header
    print_info "正在停止服务..."
    pm2 stop pm2.config.js
    print_success "服务已停止"
}

# 函数：重启服务
restart_services() {
    print_header
    print_info "正在重启服务..."
    pm2 restart pm2.config.js
    sleep 2
    pm2 status
    print_success "服务已重启"
}

# 函数：删除服务
delete_services() {
    print_header
    print_warning "正在删除 PM2 服务..."
    pm2 delete pm2.config.js || true
    print_success "PM2 服务已删除"
}

# 函数：查看状态
show_status() {
    print_header
    pm2 status
    echo ""
    print_info "详细信息:"
    pm2 info aetheris-backend
    echo ""
    pm2 info aetheris-frontend
}

# 函数：查看日志
show_logs() {
    if [ -z "$2" ]; then
        # 显示所有日志
        pm2 logs
    elif [ "$2" = "backend" ]; then
        pm2 logs aetheris-backend
    elif [ "$2" = "frontend" ]; then
        pm2 logs aetheris-frontend
    else
        print_error "未知的日志类型: $2"
        print_info "用法: ./pm2.sh logs [backend|frontend]"
        exit 1
    fi
}

# 函数：监控
show_monitor() {
    print_header
    print_info "启动 PM2 监控面板..."
    pm2 monit
}

# 函数：重新编译并重启
rebuild_and_restart() {
    print_header
    print_info "正在重新编译后端..."
    build_backend

    print_info "正在重启后端服务..."
    pm2 restart aetheris-backend

    sleep 2
    pm2 status
    print_success "后端已重新编译并重启"
}

# 函数：显示帮助
show_help() {
    print_header
    echo -e "${CYAN}使用方法:${NC}"
    echo "  ./pm2.sh [command]"
    echo ""
    echo -e "${CYAN}可用命令:${NC}"
    echo -e "  ${GREEN}start${NC}       - 启动前后端服务"
    echo -e "  ${GREEN}stop${NC}        - 停止所有服务"
    echo -e "  ${GREEN}restart${NC}     - 重启所有服务"
    echo -e "  ${GREEN}status${NC}      - 查看服务状态"
    echo -e "  ${GREEN}logs${NC}        - 查看所有日志 (Ctrl+C 退出)"
    echo -e "  ${GREEN}logs backend${NC}  - 查看后端日志"
    echo -e "  ${GREEN}logs frontend${NC} - 查看前端日志"
    echo -e "  ${GREEN}monitor${NC}     - 打开 PM2 监控面板"
    echo -e "  ${GREEN}build${NC}       - 编译后端"
    echo -e "  ${GREEN}rebuild${NC}     - 重新编译后端并重启"
    echo -e "  ${GREEN}delete${NC}      - 删除 PM2 服务"
    echo -e "  ${GREEN}help${NC}        - 显示此帮助信息"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo "  ./pm2.sh start          # 启动服务"
    echo "  ./pm2.sh logs backend   # 查看后端日志"
    echo "  ./pm2.sh rebuild        # 重新编译后端并重启"
    echo ""
}

# 主逻辑
check_pm2

case "${1:-help}" in
    start)
        start_services
        ;;
    stop)
        stop_services
        ;;
    restart)
        restart_services
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs "$@"
        ;;
    monitor|mon)
        show_monitor
        ;;
    build)
        build_backend
        ;;
    rebuild)
        rebuild_and_restart
        ;;
    delete|remove)
        delete_services
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "未知命令: $1"
        echo ""
        show_help
        exit 1
        ;;
esac
