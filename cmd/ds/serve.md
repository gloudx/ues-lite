# UES Datastore с REST API

Утилита для работы с ключ-значение хранилищем с поддержкой REST API и различных каналов связи.

## Возможности

### Локальный режим
- Работа с локальной базой данных Badger
- Все операции выполняются напрямую с файлами на диске

### Серверный режим
- REST API сервер с HTTP/HTTPS
- Unix socket для межпроцессного взаимодействия
- Graceful shutdown
- CORS поддержка
- Логирование запросов

### Удаленный режим
- Работа через HTTP API
- Поддержка Unix socket
- Автоматическое переключение между локальным и удаленным режимами

## Установка и сборка

```bash
# Клонирование репозитория
git clone <repository-url>
cd ues-lite

# Установка зависимостей
go mod download

# Сборка
go build -o ues-ds ./cmd/ds/

# Или установка
go install ./cmd/ds/
```

## Использование

### 1. Локальный режим (по умолчанию)

```bash
# Сохранить данные
ues-ds put /users/john '{"name": "John Doe", "age": 30}'

# Получить данные
ues-ds get /users/john

# Список ключей
ues-ds list --prefix=/users

# Поиск
ues-ds search john

# Статистика
ues-ds stats

# Экспорт данных
ues-ds export-jsonl --output=backup.jsonl

# Импорт данных
ues-ds load-jsonl backup.jsonl

# JS обработчики событий
ues-ds subscribe logger "console.log('Event:', event.type, event.key)"
```

### 2. Запуск сервера

```bash
# HTTP сервер на порту 8080
ues-ds serve

# Кастомный порт и хост
ues-ds serve --host=0.0.0.0 --port=3000

# Unix socket
ues-ds serve --unix-socket=/tmp/ues-ds.sock

# С дополнительными опциями
ues-ds serve --cors=true --log-requests=true
```

### 3. Работа с удаленным сервером

```bash
# Через HTTP
ues-ds --endpoint=http://localhost:8080 list
ues-ds --endpoint=http://localhost:8080 put /test "Hello Remote"
ues-ds --endpoint=http://localhost:8080 get /test

# Через Unix socket
ues-ds --endpoint=unix:///tmp/ues-ds.sock list

# С переменной окружения
export UES_ENDPOINT=http://localhost:8080
ues-ds list
ues-ds put /remote/test "Hello World"
```

## REST API

### Базовые эндпоинты

```bash
# Проверка здоровья сервера
curl http://localhost:8080/api/health

# Список ключей
curl "http://localhost:8080/api/keys?prefix=/users&limit=10"

# Получить значение ключа
curl http://localhost:8080/api/keys/users/john

# Сохранить значение (raw data)
curl -X PUT http://localhost:8080/api/keys/users/jane \
  -H "Content-Type: text/plain" \
  -d "Jane Smith"

# Сохранить значение (JSON с метаданными)
curl -X PUT http://localhost:8080/api/keys/users/bob \
  -H "Content-Type: application/json" \
  -d '{"value": "Bob Wilson", "ttl": "1h"}'

# Удалить ключ
curl -X DELETE http://localhost:8080/api/keys/users/bob

# Информация о ключе
curl http://localhost:8080/api/keys/users/john/info
```

### Поиск и статистика

```bash
# Поиск ключей
curl -X POST http://localhost:8080/api/search \
  -H "Content-Type: application/json" \
  -d '{"query": "user", "case_sensitive": false, "limit": 10}'

# Статистика
curl http://localhost:8080/api/stats

# Очистка (требует подтверждения)
curl -X DELETE "http://localhost:8080/api/clear?confirm=true"
```

### JavaScript подписки

```bash
# Список подписок
curl http://localhost:8080/api/subscriptions

# Создать подписку
curl -X POST http://localhost:8080/api/subscriptions \
  -H "Content-Type: application/json" \
  -d '{
    "id": "webhook-handler",
    "script": "console.log(\"New event:\", event.type, event.key); if (event.type === \"put\") { HTTP.post(\"http://webhook.site/unique-id\", {key: event.key, value: event.value}); }",
    "execution_timeout": 10,
    "enable_networking": true,
    "enable_logging": true,
    "event_filters": ["put", "delete"]
  }'

# Удалить подписку
curl -X DELETE http://localhost:8080/api/subscriptions/webhook-handler
```

## Конфигурация

### Переменные окружения

```bash
# Локальный режим
export UES_DATA_DIR=/var/lib/ues-ds

# Удаленный режим
export UES_ENDPOINT=http://localhost:8080

# Серверный режим
export UES_SERVER_HOST=0.0.0.0
export UES_SERVER_PORT=8080
export UES_UNIX_SOCKET=/tmp/ues-ds.sock
```

### Systemd сервис

```ini
# /etc/systemd/system/ues-ds.service
[Unit]
Description=UES Datastore Server
After=network.target

[Service]
Type=simple
User=ues-ds
Group=ues-ds
WorkingDirectory=/var/lib/ues-ds
Environment=UES_DATA_DIR=/var/lib/ues-ds/data
ExecStart=/usr/local/bin/ues-ds serve --host=127.0.0.1 --port=8080
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# Установка и запуск
sudo systemctl daemon-reload
sudo systemctl enable ues-ds
sudo systemctl start ues-ds

# Проверка статуса
sudo systemctl status ues-ds

# Логи
sudo journalctl -u ues-ds -f
```

## Примеры интеграции

### Bash скрипт

```bash
#!/bin/bash

# Конфигурация
UES_ENDPOINT="http://localhost:8080"

# Функция для работы с API
ues_api() {
  local method="$1"
  local endpoint="$2"
  local data="$3"
  
  if [ -n "$data" ]; then
    curl -s -X "$method" "$UES_ENDPOINT/api$endpoint" \
      -H "Content-Type: application/json" \
      -d "$data"
  else
    curl -s -X "$method" "$UES_ENDPOINT/api$endpoint"
  fi
}

# Примеры использования
ues_api GET "/health"
ues_api PUT "/keys/config/app" '{"value": "{\"debug\": true}"}'
ues_api GET "/keys/config/app"
```

### Python клиент

```python
import requests
import json

class UESClient:
    def __init__(self, base_url="http://localhost:8080"):
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()
    
    def get(self, key):
        response = self.session.get(f"{self.base_url}/api/keys{key}")
        if response.status_code == 404:
            return None
        response.raise_for_status()
        return response.json()['data']['value']
    
    def put(self, key, value, ttl=None):
        data = {"value": value}
        if ttl:
            data["ttl"] = ttl
        
        response = self.session.put(
            f"{self.base_url}/api/keys{key}",
            json=data
        )
        response.raise_for_status()
    
    def delete(self, key):
        response = self.session.delete(f"{self.base_url}/api/keys{key}")
        response.raise_for_status()
    
    def list_keys(self, prefix="/", limit=None):
        params = {"prefix": prefix}
        if limit:
            params["limit"] = limit
            
        response = self.session.get(f"{self.base_url}/api/keys", params=params)
        response.raise_for_status()
        return response.json()['data']['keys']

# Использование
client = UESClient()
client.put("/users/alice", "Alice Smith")
value = client.get("/users/alice")
keys = client.list_keys("/users")
```

### Node.js клиент

```javascript
const axios = require('axios');

class UESClient {
  constructor(baseURL = 'http://localhost:8080') {
    this.client = axios.create({ baseURL });
  }

  async get(key) {
    try {
      const response = await this.client.get(`/api/keys${key}`);
      return response.data.data.value;
    } catch (error) {
      if (error.response?.status === 404) return null;
      throw error;
    }
  }

  async put(key, value, ttl = null) {
    const data = { value };
    if (ttl) data.ttl = ttl;
    
    await this.client.put(`/api/keys${key}`, data);
  }

  async delete(key) {
    await this.client.delete(`/api/keys${key}`);
  }

  async listKeys(prefix = '/', limit = null) {
    const params = { prefix };
    if (limit) params.limit = limit;
    
    const response = await this.client.get('/api/keys', { params });
    return response.data.data.keys;
  }

  async createSubscription(id, script, options = {}) {
    await this.client.post('/api/subscriptions', {
      id,
      script,
      execution_timeout: 5,
      enable_logging: true,
      enable_networking: true,
      ...options
    });
  }
}

// Использование
const client = new UESClient();

(async () => {
  await client.put('/config/debug', 'true');
  const value = await client.get('/config/debug');
  console.log('Debug mode:', value);
  
  // Webhook обработчик
  await client.createSubscription('webhook', `
    if (event.type === 'put' && event.key.startsWith('/users/')) {
      HTTP.post('https://hooks.slack.com/webhook', {
        text: 'New user added: ' + event.key
      });
    }
  `);
})();
```

## Мониторинг и отладка

### Логи сервера

```bash
# Запуск с подробными логами
ues-ds serve --log-requests=true

# Логи в файл
ues-ds serve 2>&1 | tee ues-ds.log

# JSON логи для структурированного парсинга
ues-ds serve --json-logs 2>&1 | jq .
```

### Метрики

```bash
# Статистика через API
curl http://localhost:8080/api/stats | jq .

# Мониторинг через CLI
ues-ds --endpoint=http://localhost:8080 stats

# Проверка здоровья
curl -f http://localhost:8080/api/health || echo "Server is down"
```

### Производительность

```bash
# Бенчмарк записи
time for i in {1..1000}; do 
  ues-ds put "/bench/$i" "test data $i" 
done

# Бенчмарк через API
time for i in {1..1000}; do 
  curl -X PUT "http://localhost:8080/api/keys/bench/$i" -d "test data $i"
done

# Мониторинг использования памяти и CPU
htop -p $(pgrep ues-ds)
```

## Безопасность

### Рекомендации для продакшена

1. **Сеть**: Используйте HTTPS и ограничьте доступ через firewall
2. **Аутентификация**: Реализуйте middleware для проверки токенов
3. **Unix Socket**: Предпочтительнее для локальных подключений
4. **Файловые права**: Ограничьте доступ к директории данных
5. **JS Sandboxing**: Отключите networking для недоверенных скриптов

### Пример конфигурации с обратным прокси

```nginx
# /etc/nginx/sites-available/ues-ds
server {
    listen 443 ssl http2;
    server_name datastore.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Базовая аутентификация
    auth_basic "UES Datastore";
    auth_basic_user_file /etc/nginx/.htpasswd;
}
```

## Troubleshooting

### Частые проблемы

**Проблема**: `connection refused` при подключении к серверу
```bash
# Проверьте, что сервер запущен
ps aux | grep ues-ds
netstat -tulpn | grep 8080

# Проверьте логи
ues-ds serve --log-requests=true
```

**Проблема**: `permission denied` для Unix socket
```bash
# Проверьте права доступа
ls -la /tmp/ues-ds.sock
chmod 666 /tmp/ues-ds.sock
```

**Проблема**: Высокое использование памяти
```bash
# Мониторинг
ues-ds stats

# Очистка старых данных
ues-ds clear --force

# Компактификация базы (если поддерживается)
ues-ds compact
```

**Проблема**: JS скрипты не выполняются
```bash
# Проверьте синтаксис
node -c script.js

# Посмотрите логи подписок
ues-ds --endpoint=http://localhost:8080 subscribe list --verbose
```