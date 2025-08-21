# Deblock – Ethereum TX Watcher (Go + Watermill + Kafka)

**Deblock Watcher** is a Go microservice that tails Ethereum in real time, filters transactions for **500k tracked addresses**, 
computes **amounts & fees**, and emits **JSON domain events** to Kafka via **Watermill CQRS EventBus**. It’s built for high **throughput 
and correctness**: automatic WS→HTTP failover, **reorg detection & replay**, and **checkpointed backfill** so no activity is lost 
during node or service downtime.

## Quickstart

1. Specify your Infura node KEY in `.env` file

2. Use terminal to start up the service.

```bash
make up               # builds and starts: watcher + kafka + Kafka UI
```

## Shutdown

```bash
make down             # stops and removes all containers
```

## Tests

```bash
make test             # runs unit tests
make test-kafka KAFKA_BROKERS=127.0.0.1:29092 # runs unit e2e tests
```

## Logs
```bash
make logs             
```

## Kafka UI
- Access Kafka UI [here](http://localhost:29093)


## Architecture
```mermaid
flowchart LR
  subgraph ETH["Ethereum RPC (WS + HTTP)"]
    NH[eth_subscribe:newHeads] --> HS[Heads Source]
    GB[eth_getBlockByNumber] --> BLK[Block Fetcher]
    RC[eth_getTransactionReceipt (batch)] --> PROC[Processor]
  end

  HS --> FZ[Finalizer (N confirmations)]
  FZ --> BLK
  BLK --> RM[Reorg Manager]
  RM -->|canonical| PROC
  PROC --> EB[Watermill CQRS EventBus]
  EB --> K[(Kafka topic)]

  subgraph PERS["Durability"]
    CP[(Checkpoint: last finalized)]
    BF[Backfill on startup]
  end
  FZ --> CP
  CP --> BF --> BLK

  subgraph OBS["Observability"]
    HZ[/healthz/]
    MX[/metrics (Prometheus)/]
    LOG[Structured logs]
  end
  HS --> OBS
  PROC --> OBS
```

## How would I handle edge cases
1. **Retries & transient failures:** RPC and Kafka ops use timeouts with exponential backoff/jitter; 
WS auto-reconnects and falls back to HTTP polling. Non-retryable errors fail fast; persistent publish failures go to a DLQ.

2. **Block reorganization:** Compare parentHash(N) to stored hash(N-1) to detect reorgs. 
On mismatch, find a common ancestor within REORG_DEPTH, rewind, reprocess, and mark events reorged=true.

3. **No data loss after 1h downtime:** Persist the last finalized block as a checkpoint. 
On restart, backfill [checkpoint+1 … (head - confirmations)] and resume streaming.

4. **Rate limits & backpressure:** Batch receipt calls with bounded concurrency and token-bucket throttling. 
On 429/timeout, back off and temporarily reduce concurrency.

5. **Idempotency & ordering:** Partition Kafka by user_id to keep per-user order. 
Consumers dedupe by user_id + tx_hash + direction (with reorged indicating replays).

6. **Deep reorgs/anomalies:** If no ancestor is found within REORG_DEPTH, alert and resync from a safe height (e.g., target - REORG_DEPTH). 
Using N confirmations minimizes the chance and blast radius.
