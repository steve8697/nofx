# 🐳 Dockerワンクリックデプロイガイド

このガイドは、Dockerを使用してAETHERIS AIトレーディング競争システムを迅速にデプロイする方法を説明します。

## 📋 前提条件

開始する前に、システムに以下が必要です：

- **Docker**: バージョン20.10以上
- **Docker Compose**: バージョン2.0以上

### Dockerのインストール

#### macOS / Windows
[Docker Desktop](https://www.docker.com/products/docker-desktop/)をダウンロードしてインストール

#### Linux (Ubuntu/Debian)

> #### Docker Composeバージョンに関する注意
>
> **新規ユーザー推奨：**
> - **Docker Desktopを使用**: 最新のDocker Composeが自動的に含まれ、別途インストールは不要
> - シンプルなインストール、ワンクリックセットアップ、GUI管理を提供
> - macOS、Windows、一部のLinuxディストリビューションをサポート
>
> **既存ユーザー向け注意：**
> - **スタンドアロンdocker-composeの非推奨**: 独立したDocker Composeバイナリのダウンロードは推奨されません
> - **組み込みバージョンを使用**: Docker 20.10+には`docker compose`コマンド（スペース付き）が含まれています
> - 古い`docker-compose`をまだ使用している場合は、新しい構文にアップグレードしてください

*推奨：Docker Desktop（利用可能な場合）またはCompose組み込みのDocker CEを使用*

```bash
# Dockerをインストール（composeを含む）
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# dockerグループにユーザーを追加
sudo usermod -aG docker $USER
newgrp docker

# インストールを確認（新しいコマンド）
docker --version
docker compose --version  # Docker 24+にはこれが含まれており、別途インストール不要
```

## 🚀 クイックスタート（3ステップ）

### ステップ1：設定ファイルを準備

```bash
# 設定テンプレートをコピー
cp config.json.example config.json

# APIキーで設定ファイルを編集
nano config.json  # または他のエディタを使用
```

**必須フィールド：**
```json
{
  "traders": [
    {
      "id": "my_trader",
      "name": "My AI Trader",
      "ai_model": "deepseek",
      "binance_api_key": "YOUR_BINANCE_API_KEY",       // ← BinanceのAPIキー
      "binance_secret_key": "YOUR_BINANCE_SECRET_KEY", // ← Binanceのシークレットキー
      "deepseek_key": "YOUR_DEEPSEEK_API_KEY",         // ← DeepSeekのAPIキー
      "initial_balance": 1000.0,
      "scan_interval_minutes": 3
    }
  ],
  "use_default_coins": true,
  "api_server_port": 3636
}
```

### ステップ2：ワンクリック起動

```bash
# すべてのサービスをビルドして起動（初回実行）
docker compose up -d --build

# 以降の起動（リビルドなし）
docker compose up -d
```

**起動オプション：**
- `--build`: Dockerイメージをビルド（初回実行またはコード更新後に使用）
- `-d`: デタッチモードで実行（バックグラウンド）

### ステップ3：システムにアクセス

デプロイが完了したら、ブラウザを開いて以下にアクセス：

- **Webインターフェース**: http://localhost:3000
- **APIヘルスチェック**: http://localhost:3636/health

## 📊 サービス管理

### 実行状態を表示

```bash
# すべてのコンテナステータスを表示
docker compose ps

# サービスヘルスステータスを表示
docker compose ps --format json | jq
```

### ログを表示

```bash
# すべてのサービスログを表示
docker compose logs -f

# バックエンドログのみを表示
docker compose logs -f backend

# フロントエンドログのみを表示
docker compose logs -f frontend

# 最後の100行を表示
docker compose logs --tail=100
```

### サービスを停止

```bash
# すべてのサービスを停止（データを保持）
docker compose stop

# コンテナを停止して削除（データを保持）
docker compose down

# コンテナとボリュームを停止して削除（すべてのデータをクリア）
docker compose down -v
```

### サービスを再起動

```bash
# すべてのサービスを再起動
docker compose restart

# バックエンドのみを再起動
docker compose restart backend

# フロントエンドのみを再起動
docker compose restart frontend
```

### サービスを更新

```bash
# 最新のコードをプル
git pull

# リビルドして再起動
docker compose up -d --build
```

## 🔧 高度な設定

### ポートを変更

`docker-compose.yml`を編集してポートマッピングを変更：

```yaml
services:
  backend:
    ports:
      - "3636:3636"  # "your_port:3636"に変更

  frontend:
    ports:
      - "3000:80"    # "your_port:80"に変更
```

### リソース制限

`docker-compose.yml`にリソース制限を追加：

```yaml
services:
  backend:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

### 環境変数

`.env`ファイルを作成して環境変数を管理：

```bash
# .env
TZ=Asia/Tokyo
BACKEND_PORT=3636
FRONTEND_PORT=3000
```

次に`docker-compose.yml`で使用：

```yaml
services:
  backend:
    ports:
      - "${BACKEND_PORT}:3636"
```

## 📁 データの永続化

システムは自動的にデータをローカルディレクトリに永続化します：

- `./decision_logs/`: AI判断ログ
- `./coin_pool_cache/`: コインプールキャッシュ
- `./config.json`: 設定ファイル（マウント済み）

**データの場所：**
```bash
# データディレクトリを表示
ls -la decision_logs/
ls -la coin_pool_cache/

# データをバックアップ
tar -czf backup_$(date +%Y%m%d).tar.gz decision_logs/ coin_pool_cache/ config.json

# データを復元
tar -xzf backup_20241029.tar.gz
```

## 🐛 トラブルシューティング

### コンテナが起動しない

```bash
# 詳細なエラーメッセージを表示
docker compose logs backend
docker compose logs frontend

# コンテナステータスを確認
docker compose ps -a

# リビルド（キャッシュをクリア）
docker compose build --no-cache
```

### ポートが既に使用中

```bash
# ポートを使用しているプロセスを検索
lsof -i :3636  # バックエンドポート
lsof -i :3000  # フロントエンドポート

# プロセスを強制終了
kill -9 <PID>
```

### 設定ファイルが見つからない

```bash
# config.jsonが存在することを確認
ls -la config.json

# 存在しない場合、テンプレートをコピー
cp config.json.example config.json
```

### ヘルスチェックが失敗

```bash
# ヘルスステータスを確認
docker inspect aetheris-backend | jq '.[0].State.Health'
docker inspect aetheris-frontend | jq '.[0].State.Health'

# ヘルスエンドポイントを手動でテスト
curl http://localhost:3636/health
curl http://localhost:3000/health
```

### フロントエンドがバックエンドに接続できない

```bash
# ネットワーク接続を確認
docker compose exec frontend ping backend

# バックエンドサービスが実行中か確認
docker compose exec frontend wget -O- http://backend:3636/health
```

### Dockerリソースをクリーン

```bash
# 未使用のイメージをクリーン
docker image prune -a

# 未使用のボリュームをクリーン
docker volume prune

# すべての未使用リソースをクリーン（注意して使用）
docker system prune -a --volumes
```

## 🔐 セキュリティ推奨事項

1. **config.jsonをGitにコミットしない**
   ```bash
   # config.jsonが.gitignoreに含まれていることを確認
   echo "config.json" >> .gitignore
   ```

2. **機密データには環境変数を使用**
   ```yaml
   # docker-compose.yml
   services:
     backend:
       environment:
         - BINANCE_API_KEY=${BINANCE_API_KEY}
         - BINANCE_SECRET_KEY=${BINANCE_SECRET_KEY}
   ```

3. **APIアクセスを制限**
   ```yaml
   # ローカルアクセスのみを許可
   services:
     backend:
       ports:
         - "127.0.0.1:3636:3636"
   ```

4. **イメージを定期的に更新**
   ```bash
   docker compose pull
   docker compose up -d
   ```

## 🌐 本番環境デプロイ

### Nginxリバースプロキシの使用

```nginx
# /etc/nginx/sites-available/aetheris
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /api/ {
        proxy_pass http://localhost:3636/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### HTTPSの設定（Let's Encrypt）

```bash
# Certbotをインストール
sudo apt-get install certbot python3-certbot-nginx

# SSL証明書を取得
sudo certbot --nginx -d your-domain.com

# 自動更新
sudo certbot renew --dry-run
```

### Docker Swarmの使用（クラスタデプロイ）

```bash
# Swarmを初期化
docker swarm init

# スタックをデプロイ
docker stack deploy -c docker-compose.yml aetheris

# サービスステータスを表示
docker stack services aetheris

# サービスをスケール
docker service scale aetheris_backend=3
```

## 📈 監視＆ロギング

### ログ管理

```bash
# ログローテーションを設定（docker-compose.ymlで既に設定済み）
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"

# ログ統計を表示
docker compose logs --timestamps | wc -l
```

### 監視ツール統合

Prometheus + Grafanaで監視を統合：

```yaml
# docker-compose.yml（監視サービスを追加）
services:
  prometheus:
    image: prom/prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana
    ports:
      - "3001:3000"
```

## 🆘 ヘルプを取得

- **GitHub Issues**: [Issueを提出](https://github.com/yourusername/open-aetheris/issues)
- **ドキュメント**: [README.md](README.md)を確認
- **コミュニティ**: Discord/Telegramグループに参加

## 📝 コマンドチートシート

```bash
# 起動
docker compose up -d --build       # ビルドして起動
docker compose up -d               # 起動（リビルドなし）

# 停止
docker compose stop                # サービスを停止
docker compose down                # コンテナを停止して削除
docker compose down -v             # コンテナとデータを停止して削除

# 表示
docker compose ps                  # ステータスを表示
docker compose logs -f             # ログを表示
docker compose top                 # プロセスを表示

# 再起動
docker compose restart             # すべてのサービスを再起動
docker compose restart backend     # バックエンドを再起動

# 更新
git pull && docker compose up -d --build

# クリーン
docker compose down -v             # すべてのデータをクリア
docker system prune -a             # Dockerリソースをクリーン
```

---

🎉 おめでとうございます！AETHERIS AIトレーディング競争システムのデプロイに成功しました！

問題が発生した場合は、[トラブルシューティング](#-トラブルシューティング)セクションを確認するか、Issueを提出してください。
