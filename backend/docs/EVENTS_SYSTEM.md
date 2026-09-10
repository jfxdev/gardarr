# Sistema de Eventos - Gardarr

## Visão Geral

O sistema de eventos rastreia automaticamente mudanças de estado dos torrents através de um poller dedicado. Todos os eventos são armazenados em banco de dados com timestamps precisos e podem ser consultados via API.

## Tipos de Eventos

As constantes de tipos de eventos estão definidas em `internal/constants/event.go`:

```go
const (
    EventTypeTorrentStateChange = "torrent.state_change"
    EventTypeTorrentAdded       = "torrent.added"
    EventTypeTorrentRemoved     = "torrent.removed"
    EventTypeTorrentCompleted             = "torrent.completed"
    EventTypeBandwidthScheduleApplied     = "bandwidth.schedule_applied"
    EventTypeTransferReportDaily          = "report.transfer.daily"
    EventTypeTransferReportWeekly         = "report.transfer.weekly"
)
```

### `torrent.state_change`
Disparado quando o estado de um torrent muda (ex: `DOWNLOADING` → `UPLOADING`)
- **Campos**: `old_value`, `new_value`, `task_hash`
- **Metadata**: `name`, `old_progress`, `new_progress`

### `torrent.added`
Disparado quando um novo torrent é detectado
- **Campos**: `new_value` (estado inicial), `task_hash`
- **Metadata**: `name`, `progress`

### `torrent.removed`
Disparado quando um torrent não é mais encontrado
- **Campos**: `old_value` (último estado), `task_hash`
- **Metadata**: `last_progress`

### `torrent.completed`
Disparado quando um torrent atinge 100% de progresso
- **Campos**: `new_value` (estado atual), `task_hash`
- **Metadata**: `name`

### `bandwidth.schedule_applied`
Disparado quando Gardarr aplica um limite de banda programado ou restaura o limite padrão do worker.
- **Campos**: `old_value`, `new_value`
- **Metadata**: `source`, `download_limit`, `upload_limit`, `old_limits`; quando a origem for uma programação, também `schedule_uuid`, `schedule_name`

### `report.transfer.daily` e `report.transfer.weekly`
Emitidos ao concluir os rankings de tráfego. São eventos globais e não têm
`worker_id`. O metadata contém período, fuso, cobertura, workers
indisponíveis e listas `upload`/`download` com os itens classificados.

## Estrutura de Dados

### Event Entity
```go
type Event struct {
    UUID      uuid.UUID
    WorkerID  uuid.UUID
    Type      string // Usar constantes de constants.EventType*
    TaskHash  string
    OldValue  string
    NewValue  string
    Metadata  map[string]interface{}
    CreatedAt time.Time
}
```

## API Endpoints

### `GET /v1/events`
Lista eventos com filtros opcionais

**Query Parameters:**
- `worker_id` (string, opcional): UUID do worker
- `type` (string, opcional): Tipo do evento
- `limit` (int, default: 50, max: 200): Número de resultados
- `offset` (int, default: 0): Offset para paginação

**Exemplo:**
```bash
curl -X GET "http://localhost:3200/v1/events?worker_id=abc-123&type=torrent.state_change&limit=10"
```

**Response:**
```json
{
  "events": [
    {
      "uuid": "event-uuid",
      "worker_id": "worker-uuid",
      "type": "torrent.state_change",
      "task_hash": "abc123",
      "old_value": "DOWNLOADING",
      "new_value": "UPLOADING",
      "metadata": {
        "name": "Ubuntu 22.04",
        "old_progress": 0.95,
        "new_progress": 1.0
      },
      "created_at": "2025-11-09T20:00:00Z"
    }
  ],
  "total": 1
}
```

### `GET /v1/events/:uuid`
Obtém um evento específico por UUID

**Exemplo:**
```bash
curl -X GET "http://localhost:3200/v1/events/event-uuid"
```

## Integração com Event Poller

O sistema de eventos é alimentado pelo serviço de event poller (`internal/services/eventpoller`):

```go
// Em pollWorker(ctx, w, now):
if err := s.eventService.TrackTasks(ctx, tasks, w.UUID, now); err != nil {
    logger.Debug("event poller: track tasks error", ...)
}
if err := s.eventService.DetectRemovedTasks(ctx, tasks, w.UUID, now); err != nil {
    logger.Debug("event poller: detect removed tasks error", ...)
}
```

- **Polling Interval**: Configurado via `EVENT_POLL_INTERVAL` (default: 30s)
- **Detecção Automática**: Compara estados a cada ciclo de polling
- **Performance**: Execução concorrente por worker

## Retenção de Dados

### Configuração via Variável de Ambiente
```env
# Dias para manter eventos (default: 30, 0 = sem limite)
EVENT_RETENTION_DAYS=30
```

O serviço lê automaticamente esta configuração na inicialização:
```go
func NewService(db *database.Database) *Service {
    return &Service{
        retentionDays: env.Get("EVENT_RETENTION_DAYS").Default(30).ValueInt(),
        // ...
    }
}
```

### Exemplos de Configuração
```env
# Manter eventos por 7 dias (ideal para ambientes de teste)
EVENT_RETENTION_DAYS=7

# Manter eventos por 90 dias (recomendado para produção)
EVENT_RETENTION_DAYS=90

# Manter eventos indefinidamente
EVENT_RETENTION_DAYS=0
```

### Limpeza Automática
O serviço oferece método para purgar eventos antigos baseado na configuração:
```go
eventService.PurgeOldEvents(ctx) // Remove eventos mais antigos que EVENT_RETENTION_DAYS
```

### Limpeza de Estados em Memória
Estados de torrents inativos (>24h) são automaticamente removidos:
```go
eventService.CleanStaleStates() // Remove estados não vistos em 24h
```

## Arquitetura

### Componentes

1. **Constants** (`internal/constants/event.go`)
   - Define constantes de tipos de eventos

2. **Entity** (`internal/entities/event.go`)
   - Define estrutura do evento

3. **Model** (`internal/models/event_model.go`)
   - Mapeamento para banco de dados (GORM)

4. **Repository** (`internal/repository/event/repository.go`)
   - Operações de persistência
   - Não deve conter lógica de negócio

5. **Service** (`internal/services/events/service.go`)
   - Lógica de rastreamento de estados
   - Detecção de mudanças
   - Cache em memória de estados
   - Delega persistência ao repository

6. **Routes** (`internal/routes/api/v1/events/routes.go`)
   - Endpoints REST

7. **Migration** (`009_create_events_table`)
   - Criação da tabela de eventos

## Uso Futuro: Webhooks

A estrutura está preparada para webhooks:

```go
// Exemplo de implementação futura
type WebhookConfig struct {
    URL    string
    Events []string // Usar constants.EventType*
}

func (s *Service) NotifyWebhooks(event *Event) {
    // Enviar evento para webhooks configurados
}
```

### Possíveis Notificações
- Torrent completado → notificar Plex para atualizar biblioteca
- Estado mudou → dashboard em tempo real
- Torrent removido → limpeza de arquivos
- Torrent adicionado → logs/auditoria

## Performance

### Estado em Memória
- Estados de torrents mantidos em `map[string]*TaskState`
- Acesso O(1) para verificação de mudanças
- Sincronização com `sync.RWMutex`

### Concorrência Multi-Nível
1. **Nível Worker**: Cada worker processado em goroutine separada
2. **Nível Task**: Cada task processada concorrentemente
   - `TrackTasks`: Goroutines para cada tarefa
   - `DetectRemovedTasks`: Goroutines para cada remoção
3. **Buffered Channels**: Eventos coletados em canais
4. **Lock Granular**: RWMutex minimiza contenção

### Otimizações
- ✅ Processamento paralelo de tarefas
- ✅ Lock apenas quando necessário (read/write separados)
- ✅ Canais buffered para coleta de eventos
- ✅ Criação de eventos em batch
- ✅ Sem impacto no desempenho do polling

### Índices de Banco
```sql
CREATE INDEX idx_events_worker_id ON events(worker_id);
CREATE INDEX idx_events_type ON events(type);
CREATE INDEX idx_events_task_hash ON events(task_hash);
CREATE INDEX idx_events_created_at ON events(created_at);
```

## Configuração Recomendada

### Arquivo .env
```env
# Eventos - Sistema de rastreamento
EVENT_RETENTION_DAYS=30        # Retenção de eventos (dias)
EVENT_POLL_INTERVAL=30s        # Intervalo de polling do event poller
```

### Recomendações por Ambiente

**Desenvolvimento**:
```env
EVENT_RETENTION_DAYS=7
EVENT_POLL_INTERVAL=60s
```

**Produção**:
```env
EVENT_RETENTION_DAYS=90
EVENT_POLL_INTERVAL=30s
```

**Heavy Load** (muitos torrents):
```env
EVENT_RETENTION_DAYS=30
EVENT_POLL_INTERVAL=30s
```

## Troubleshooting

### Eventos não sendo criados
1. Verificar logs do event poller
2. Confirmar que migration foi aplicada: `009_create_events_table`
3. Verificar se `EVENT_RETENTION_DAYS` não é 0 (se usar purge automático)

### Muitos eventos gerados
1. Ajustar `EVENT_POLL_INTERVAL` para polling menos frequente
2. Implementar filtros no frontend
3. Configurar retenção menor: `EVENT_RETENTION_DAYS=7`
4. Implementar purge automático agendado

### Performance degradada
1. Verificar tamanho da tabela de eventos
2. Executar limpeza: `eventService.PurgeOldEvents(ctx)`
3. Verificar índices do banco de dados
4. Considerar reduzir `EVENT_RETENTION_DAYS`

## Exemplos de Uso

### Frontend - Página de Histórico
```typescript
// Buscar eventos recentes
const response = await api.get('/events', {
  params: {
    limit: 20,
    offset: 0
  }
});

// Filtrar por worker específico
const workerEvents = await api.get('/events', {
  params: {
    worker_id: selectedWorker.uuid,
    type: 'torrent.state_change'
  }
});
```

### Monitoramento de Completados
```typescript
// Buscar torrents que completaram hoje
const today = new Date().toISOString().split('T')[0];
const completed = await api.get('/events', {
  params: {
    type: 'torrent.completed',
    // Adicionar filtro de data no backend se necessário
  }
});
```

## Próximos Passos

1. ✅ Sistema de eventos básico implementado
2. 🔄 Criar página de histórico no frontend
3. 📋 Implementar webhooks
4. 📊 Adicionar métricas e analytics
5. 🔔 Notificações em tempo real (WebSocket)
6. 🎯 Filtros avançados (por data, múltiplos workers, etc)
7. 📁 Export de eventos (CSV, JSON)
