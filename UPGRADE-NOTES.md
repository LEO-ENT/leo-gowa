# Upgrade LEO GoWA a upstream v9.6.0

## Baseline e strategia

Fork origin/main `5e42fc5`; tag upstream v9.6.0 `62e319d`. Lo stato iniziale era un rebase interattivo interrotto al primo di 29 commit, non un merge. Il rebase e stato annullato dopo aver conservato diff e staged diff nella metadata Git locale; integrazione tramite merge del tag nel solo branch `feat/gowa-upstream-upgrade`. La storia LEO rimane raggiungibile, senza rebase di main. Nessun rilascio o dato reale utilizzato.

## Compatibilita REST, webhook, configurazione

- MCP passa da processo/subcommand separato a endpoint `/mcp` del server `rest`; `MCP_HOST`/`MCP_PORT` non servono piu. I tool sono consolidati per dominio, con selettori `type`/`action`: i client MCP devono aggiornare nomi e parametri. Basic Auth resta il default; OAuth 2.1 e opt-in (`MCP_OAUTH_ENABLED=false`), con issuer/resource HTTPS e SQLite dedicato, documentati in `docs/mcp-oauth.md`. `MCP_ENABLED=true` e il default.
- REST mantiene device scope e gli endpoint LEO provider. Aggiunge `/message/:message_id/played`, `/newsletter/messages/:server_id/download`, `/chat/:chat_jid/history`, scheduled/recurring sends e `/send/schedules`. I send possono restituire `schedule_id`, `scheduled_at`, `next_run_at` anziche `message_id` quando programmati; preservati ID deterministici, timeout e ricevute ACK dei send immediati. Aggiunti HD media, mentions nei caption, `allow_reshare` solo status e `link` negli edit. Purge device e decoding parametri seguono upstream.
- MCP, UI e Chatwoot config sono montati prima del gruppo device: Fiber applica middleware anche alle route successive. Il helper LEO provider/device resta ultima composizione, con guard AST e test avversario che impedisce route tardive.
- B1: `sender_display_name` upstream RIMOSSO da tutti i payload webhook (messaggi, media, poll, voti, reaction, revoke/edit, receipt, delete, chat presence, call offer), senza flag: porta nomi della rubrica e rompe la whitelist `user_text` dell'orchestrator. Contract test `webhook_no_sender_display_name_test.go`; `from_name` rimane compatibile. Il campo resta solo nelle risposte REST di chat/messages. Template/business/buttons/product/selection ricevono struttura e testo; reply/referral metadata sono persistiti. I consumer devono tollerare campi nuovi. Payload `poll` di #816 nativo upstream ha gli stessi campi, hash, array vuoti e resolution statuses del backport LEO: golden fixtures NON modificate. L'evento rimane `message`.
- `WHATSAPP_IGNORE_STATUS_MEDIA=false` e `WHATSAPP_WEBHOOK_DEVICE_MERGE_GLOBAL=false` sono nuovi default. Merge-global opera solo nel routing legacy: in managed/fail-closed LEO nessuna copia globale o fallback in caso di errore configurazione. Spool cifrato, admission durable, privacy e isolamento device LEO preservati.
- `GET /app/info` restituisce versione reale `v9.6.0` e `scheduled_sends=true`; test HTTP senza device registra e verifica la versione. La soglia orchestrator >=9.3 puo quindi abilitare sondaggi.
- Toolchain upstream Go 1.26.0, nuove versioni whatsmeow/Fiber e workflow release. CI LEO preservata e allineata a `src/go.mod`, con CGO e purego. Nessun workflow pubblicazione invocato.

## Migration posizionali e riconciliazione

| Indice fork LEO | Indice upstream | Operazione |
|---|---|---|
| 1-43 | 1-43 | Schema comune invariato |
| 44 | nessuno | managed_webhook_spool LEO |
| 45 | nessuno | unique delivery ID spool |
| 46 | nessuno | indice due/lease spool |
| 47 | nessuno | provider_message_receipts LEO |
| 48 | 44 | poll_definitions, gia applicata: NON ripetere |
| 49 | 45 | indice messages(chat_jid,device_id,timestamp) |
| 50 | 46 | scheduled_sends |
| 51 | 47 | indice schedule status/next_run_at |
| 52 | 48 | indice schedule device/status/next_run_at |
| 53 | 49 | messages.context_metadata |

Le SQL delle migration fork 1-48 sono preservate; tutte le nuove DDL sono append-only. Un DB LEO a 48 applica 49-53, ciascuna nella transazione del runner con aggiornamento schema_info. Le CREATE sono IF NOT EXISTS, ALTER TABLE 53 viene eseguita una sola volta grazie alla versione: niente manipolazione o rinumerazione dello storico. Il test `TestUpgradeHistoricalFork48PreservesData` usa `testdata/fork_v48.sql`, estratta da origin/main e indipendente dalla lista nuova; semina messages/poll_definitions, verifica sopravvivenza, schema/indici/versioni e secondo InitializeSchema senza replay. Un DB upstream puro a 48 non equivale a DB LEO48 e non e un input supportato da questo piano.

## Voti: causa accertata e punto 5

Il brief aggiornato il 11/10 riporta la causa accertata in produzione: il poll di gruppo inviato via REST dallo slot applicativo non aveva una definizione nel suo device scope dopo un errore SQLITE_BUSY/database is locked; la definizione esisteva solo per un altro device registrato nella stessa istanza e membro del gruppo. Il voto sul primo device risultava definition_missing. Questa evidenza e fornita dal brief: Codex non ha letto DB o log di produzione.

Fix (a): persistSentPollDefinition (src/usecase/send.go:1618) conserva il timestamp del send e ritenta esclusivamente la persistenza, mai l'invio WhatsApp. Usa l'UPSERT atomico con ExecContext (src/infrastructure/chatstorage/sqlite_repository.go:903), classifier SQLite BUSY/LOCKED compatibile CGO/purego, massimo 8 tentativi, backoff 50/100/200/400/800ms e budget context 10s. Il contesto HTTP e detached, mantenendo device scope. Errori non transient non vengono ritentati. Send gia confermato rimane 200; se esaurisce i tentativi emette warning strutturato con operation/attempts/failure, senza ID/JID/backend. Il busy handler nativo CGO configurato a 30s puo ritardare la restituzione di un singolo ExecContext oltre la deadline: il retry resta finito ma il limite di wall time dipende dal driver.

Fix (b): dopo lookup esatto e lookup PN/LID dello stesso device, se manca la definizione e DecryptPollVote riesce, loadRegisteredPollDefinition (src/infrastructure/whatsapp/poll_event.go:194, chiamata dopo decrypt:300) richiede metadata dello stesso gruppo e poll ID da un device registrato. GetPollDefinitionForRegisteredCreator (src/infrastructure/chatstorage/sqlite_repository.go:1010) controlla receiver, creatore e donor in devices.jid nello stesso statement/database della istanza. Nessun lookup globale per solo ID, nessun fallback per DM o device non registrati. Definizioni discordanti fra donor sono rifiutate; copie identiche selezionano la piu recente. Nessuna modifica o copia delle righe/partition, nessuna nuova migration.

Verifica whatsmeow della versione pinned: msgsecret.go:getOrigSenderFromKey/generateMsgSecretKey/decryptMsgSecret/DecryptPollVote. La chiave deriva dal secret originale, ID poll, creatore e votante; il receiver non entra nella derivazione, ma deve disporre del secret nel proprio store. MessageKey.FromMe identifica creatore uguale al VOTANTE, non al receiver; per gruppo altrimenti e key.Participant. Il codice riproduce questa regola, normalizza PN/LID e non modifica l'evento o condivide secret tra device. Fallback solo dopo autenticazione riuscita; secret mancante resta decrypt_failed e non legge metadata altri device.

Fix (c): send_poll_persistence_test.go verifica BUSY transitorio, esaurimento, nonbusy, cancellazione HTTP, deadline e lock SQLite reale BEGIN IMMEDIATE rilasciato dopo il primo tentativo. sqlite_repository_cross_poll_test.go verifica registro receiver/creator/donor, chat/ID esatti, isolamento e ambiguita. poll_registered_fallback_test.go cifra voti reali con receiver diverso da creatore/votante, ricrea il caso creatore senza propria definizione e donor presente, copre FromMe/secret assente/creator non registrato/errori registry e verifica submission webhook resolved con device_id del receiver. Ultima chiamata rete sostituita da spy; nessun send REST/provider o dispositivo reale contattato.

Golden fixtures immutati. Il test precedente RESTSavedPollIncomingVoteReachesWebhook copre evento cifrato, definizione in spy equivalente al risultato del send REST e contesto cancellato. Warning di scarto/failure senza identificatori. Handler events.Message registrato; nessun filtro generale from_me. Il precedente rapporto causa non trovata e superato dall'evidenza esplicita aggiunta al punto 5.

## UPSERT atomico

`UpsertPollDefinition` ora esegue un solo `INSERT ... ON CONFLICT(device_id,chat_jid,poll_message_id) DO UPDATE ... WHERE excluded.updated_at > poll_definitions.updated_at`. Evita la sequenza SELECT + write in transazione deferred che poteva richiedere lock upgrade; conserva created_at e non sovrascrive eventi piu nuovi con replay vecchi/equivalenti. Il test copre timestamp older/equal/newer. Non elimina la possibilita generale SQLITE_BUSY dovuta ad altri writer.

## Patch LEO mantenute

Elenco ottenuto da `git log --oneline v9.0.0..origin/main`; include merge e backport provenienti dalle linee fork. #816 ora condiviso con upstream nativo; migration 48 preservata come indice fork. Patch principali: media device-scoped, read receipts gruppi, fail-closed per-device, managed source spool, deterministic outbound IDs, provider receipt/reconciliation e path opacity, privacy Whatsmeow/Chatwoot, portable CI e golden poll contract.

```text
5e42fc5 Merge pull request #4 from LEO-ENT/feat/poll-vote-webhook-backport
674a317 feat(webhook): backport upstream poll vote selections (#816) onto the LEO fork
5d330bb Merge PR #3: integrate path-opacity line with the group read-receipt fix
2999fdd fix(merge): restore ErrInvalidJID dropped as deadcode by the markread line
188605e Merge commit 'f8ba7596211d710c0c06f31ad7f74ab6f9943857' into prep/integration-probe
ed7b6d8 test(provider): restore lookup call graph census
d3752d0 fix(provider): deny lookup path by default
a709cfb fix(provider): bind lookup opacity to resolved route
eccd6f9 fix(provider): make lookup opacity path-owned
8ef4f3d fix(provider): enforce production middleware ordering
153b953 fix(provider): harden receipt reconciliation boundary
cc7bb47 docs(provider): publish reconciliation contract and CI gate
d7bda59 feat(provider): add device-scoped message reconciliation
75606c4 test(logging): bound receipt alias closure
cb6f36e test(logging): harden typed receipt guard
78ffc1b test(logging): enforce typed receipt boundary
de90a03 fix(logging): redact Chatwoot receipt failures
6e84a1e fix(logging): harden WhatsApp PII boundaries
8979e39 fix(logging): redact Whatsmeow client events
051b9f7 feat(send): support deterministic outbound text IDs
6c941ca fix(ci): make GoWA verification portable
b4f85d1 ci: verify GoWA pull requests
6b3bb78 fix(webhook): harden spool startup and claim contention
36ee197 feat(webhook): add durable managed source spool
e9565fe test(webhook): close generic fail-closed test scope
fe4d835 test(webhook): pin Chatwoot on generic fail-closed
86345a6 fix(webhook): fail closed per-device routing
f8ba759 fix(webhook): auto-download document payload carries the sender filename
576af45 fix: keep the request port in generated URLs (#779) (#781)
45280a8 fix(media): scope downloads to active device (#1)
b07de66 Merge branch 'main' into fix/markread-group-participant
67693d1 Merge branch 'main' into fix/markread-group-participant
e360892 address review: scope sender lookup to chat + structured logging (#772)
3547696 fix: resolve group message sender for MarkAsRead read receipts
```

## Verifica

Esiti definitivi, commit e PR sono riportati in `../CODEX-REPORT.md`. I test locali usano dati sintetici e database temporanei, senza sessioni WhatsApp reali.

### Verifica aggiuntiva del punto 5

Il warning finale di persistenza contiene campi strutturati `operation`, `attempts` e `failure` (categoria sicura, senza errore backend/JID). Il test di esaurimento verifica otto tentativi e assenza di identificatori. Il test `TestRegisteredCreatorPollFallback/creator_receives_donor_copy` passa con il fallback; una prova tramite overlay Go che disabilita soltanto il fallback fallisce con `definition_missing` invece di `resolved`. La prova non modifica i sorgenti del branch.
