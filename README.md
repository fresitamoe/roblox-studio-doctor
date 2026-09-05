# studio-doctor

Reads Roblox Studio's log files and tells you what happened in a session. Team Create
disconnections, and crashes. Can find script errors, assets that Studio couldn't fetch, can also detect playtests taking longer to load.

Nothing else is needed, this alone functions on its own. No plugin needed.

Studio writes these logs whether or not this tool is running, so it works after the fact.
If you install it after a crash, it can still tell you about said crash.

## Install

Grab a binary off the releases page, or if you already have Go:

```bash
go install github.com/Vliysl/roblox-studio-doctor/cmd/studio-doctor@latest
```

## Use

```bash
studio-doctor                  # most recent session
studio-doctor -json            # machine readable
studio-doctor -all             # every session ranked
studio-doctor -all -since 168h # just the last week
studio-doctor -log-dir /path/to/logs
```

The log directory is found automatically. `-log-dir` is there for Vinegar, Sober, or logs
copied off another machine.

`-all` prints one row per session, worst first, so you can find the bad one:

```
3 session(s) analysed, worst first.

START                BUILD            DURATION  COVERAGE  FINDINGS
2026-08-12 11:34:26  0.737.0.7371584  1h12m4s   99.8%     1 critical, 2 warn
2026-08-13 17:05:10  0.737.0.7371584  4h34m2s   99.9%     1 warn
2026-08-11 09:12:00  0.736.1.7361120  22m11s    99.9%     clean
```

Severity ranks it, recency only breaks ties. `-since` takes any Go duration and only
works with `-all`.

## What it can report

| Finding | Meaning |
| --- | --- |
| `teamcreate-lost-connection` | Dropped Team Create session. Anything edited after that might not have reached the server |
| `crash-no-clean-exit` | No shutdown sequence in the log. `info` if the log is still being written, `warn` if not |
| `script-errors` | Script errors, repeats folded together/overlapped. Sorted by whose code it is first, so your own scripts come before Roblox internals and plugins |
| `asset-access-denied` | Assets this session had no access to, with the id and type |
| `playtest-slowdown` | Playtesting loads got slower. Worst load 2.5x the first and over 10 seconds |

Every finding points at the log lines it came from. It reports what happened, it however cannot guess any root reason.

`memory-growth` is in the code but off. `AppMemUsageStatus` sends static tiers instead of
a memory series so there's nothing to measure. A hang rule got dropped for the same
reason, `DataModelANRListener` never fired at all.

## Licence

MIT
