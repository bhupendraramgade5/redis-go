## COMMANDS
### [WATCH](#watch-1)
<!-- ### [OPTIMISTIC LOCKING](#optimistic-locking) -->

## [WATCH](#watch)
```
    WATCH key [key ...]
```
1. Provides a check-and-set (CAS) behavior to Redis transactions.

2. WATCHed keys are monitored in order to detect changes against them. `If at least one watched key is modified` before the EXEC command, `the whole transaction aborts`, and EXEC returns a `Null reply` to notify that the transaction failed.

3. If there are race conditions and another client modifies the result of val in the time between our call to WATCH and our call to EXEC, the transaction will fail. We just have to `repeat the operation hoping this time we'll not get a new race`. This form of locking is called [optimistic locking.](#optimistic-locking)

✅ Actual Redis Model (very important)

WATCH works like this:

1. When client calls WATCH key

*    Server records:
```
    client → key → version_at_watch_time
```
2. When ANY client modifies that key

*   Server simply:
```
    key.version++
```
3. When EXEC is called

Server checks:
```
    if current_version != watched_version → abort
    else → execute transaction
```
👉 That’s it. No channels. No event system. No rollback.


`WATCH is just version comparison at EXEC time`

NOT:

*    1. continuous monitoring
*    2. NOT reactive
*    3. NOT event-driven