# EVTC format history

⚠️ **This research document are generated with Claude Fable 5.1 on 2026-09-16. I want to be transparant about IA usage on my projects. Every generated text have a disclaimer on top of it. Thanks for your interest !**

How the arcdps log format changed between December 2016 and September
2026, compiled from the Wayback Machine captures of the arcdps site. The
timeline reads logs of arcdps 20240613 and later, in the format of
20260501 and in the one before; this is the reference for that reading,
and for the older logs it refuses.

"The README" is the arcdps `evtc/README.txt`. A version is named by the
month of the capture that first shows it ("README of 2024-07"); the edit
behind a capture can be months older (`dirs_files.tsv`). Dates are
release dates of the arcdps changelog unless a line says "build". A kind
is a value of `is_statechange`. A number in parentheses after a name is
its value in its enum, which is `cbtstatechange` for a kind. G0 to G3 are
the [generations](#generations) of the format.

The header of a log carries the build date. Of the 49 builds of the [logs
measured](#logs), 42 are dated a release day, 2 the day after (20250730,
20260709), and 5 have no changelog entry on either day (20250907,
20250925, 20251122, 20260416, 20260718). The README dates a retirement by
a build a few days to two weeks older than the matching release (see
[Retired kinds](#retired-kinds)), so a build slightly older than a
release can already behave the new way.

## Sources

Captures are the index rows with content; redirects and revisit records
are left out.

| Page                                             | Captures                     | Distinct versions | Covers                                                                                                                          |
| ------------------------------------------------ | ---------------------------- | ----------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `deltaconnected.com/arcdps/`                     | 216                          | 183               | changelog entries from 2016-12-08 to 2026-09-15                                                                                 |
| `arcdps/evtc/` (listing)                         | 88, sorted variants included | 78                | file names, dates and sizes from 2017-06-23 to 2026-09-16; 43 dated states of the README, one of them the re-date of 2022-08-25 |
| `arcdps/evtc/README.txt`                         | 16                           | 15                | 2017-10-22, then 2022-05-24 to 2026-09-16                                                                                       |
| `arcdps/evtc/writeencounter.cpp`                 | 3                            | 1                 | 2022-05-24 to 2024-12-02                                                                                                        |
| `arcdps/evtc/example.cpp`                        | 1                            | 1                 | 2017-10-22, the ancestor of `writeencounter.cpp`                                                                                |
| `arcdps/x64/evtc/` (listing)                     | 1                            | 1                 | 2017-02-22, where the code lived first; its files were never captured                                                           |
| `arcdps/api/README.txt`, `arcdps_combatdemo.cpp` | 7                            | 6                 | 2020-12-31 to 2026-06-04; the revision 1 event struct, no enum                                                                  |

Distinct versions are distinct index digests, and distinct contents for
the README, where two captures differ only by their compression.
`writeencounter.cpp` is the published writer; the listings date its one
captured version 2022-03-19.

The main page keeps only its latest changelog entries, so the history is
the union of all versions: about 2470 entries, about 330 of them naming
evtc, over 511 release days (`changelog_all.tsv` has a few more rows, for
entries reworded between two captures). The live `README.txt` and
`writeencounter.cpp`, fetched on 2026-09-21, and the live changelog, read
on 2026-09-22, cover the days after the last capture: the changelog adds
the release of 2026-09-20, the README differs from its capture by four
`realtime:` annotations, and the live writer has an edit of 2025-08-19
that no capture holds (`addr` renamed `iid`, stats `int16_t` again after
the `uint16_t` captured in 2022).

Everything is kept under `tmp/arcdps-history/`, which git ignores:

| Path                       | Content                                                                                                                   |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `readme/`, `code/`, `api/` | every distinct version, named by capture timestamp                                                                        |
| `diffs/`                   | unified diff between consecutive README versions                                                                          |
| `pages/`, `listings/`      | the raw main pages and directory listings                                                                                 |
| `changelog_all.tsv`        | date, first capture, last capture, text of every changelog entry                                                          |
| `changelog_evtc.txt`       | the release days that touch the format, whole days for context, format lines starred                                      |
| `enums_matrix.txt`         | every enum of every README version, value by value, with renames                                                          |
| `dirs_files.tsv`           | every file date and size the listings show                                                                                |
| `index/`                   | the CDX indexes and download lists                                                                                        |
| `tools/`                   | `fetch.sh` downloads a list, `changelog.py`, `changelog_filter.py`, `enums.py` and `dirs.py` rebuild the four files above |

### Logs

The measurements of this document come from 10,573 boss and map logs of
49 builds, 20230114 to 20260920, all revision 1: 4,891 of G2, from 36
builds up to 20260416, and 5,682 of G3, from 13 builds starting with
20260507. They hold nothing older, nothing between 20230114 and 20240613
and nothing between 20240723 and 20250708. "Measured" means counted on
them build by build: the kinds a build writes, the fields of a kind that
are ever non-zero, the values of the byte fields. The logs are kept out
of git under `tmp/zevtc-fixtures/`, and
`EVTC_REAL_LOGS=tmp/zevtc-fixtures go test ./timeline -run TestRealLogs`
parses them all and builds those from 20240613 on: 10,565 logs, 1.57
billion events, no invariant failing. The same test writes each G3 log
the G2 way and checks it builds the same graph.

### Holes in the sources

- The README has no capture between 2017-10-22 and 2022-05-24. The
  listings date twelve versions of that window that were never captured
  (2017-11-08, 2018-04-24, 2018-07-05, 2018-08-23, 2018-10-23,
  2019-03-24, 2019-09-19, 2019-11-30, 2020-01-06, 2021-04-29, 2021-06-19,
  2021-08-28); there may be more, since a listing only shows the latest
  edit before it. What they said is only known through the changelog and
  the README of 2022-05, so a fact credited to that README can be as old
  as 2017-11.
- Same between 2025-05-23 and 2026-05-10: six versions never captured
  (2025-05-26, 2025-06-03, 2025-08-04, 2025-08-29, 2025-11-14,
  2026-04-04). Kinds 60 to 63 were released eleven months before the
  first captured README that documents them, 64 to 66 eight months.
- Smaller gaps separate the other captures; `dirs_files.tsv` dates every
  version a listing saw.
- `writeencounter.cpp` of 2018-09-01 (4.6K) and 2018-10-02 (3.5K) were
  never captured; they span the move to revision 1. Neither were the
  `example.cpp` of 2017-03-13 and 2018-07-05.
- The listing of 2022-10-04 shows every file re-dated to 2022-08-25
  23:40, and the times move by four hours between the listings of 2023-06
  and 2023-10.
- The changelog may miss entries of 2016-12-22 to 2017-02-02 and
  2017-02-22 to 2017-03-26: no capture in those weeks and the list had
  rolled over by the next one. Shorter windows with the same risk:
  2017-04-20 to 2017-04-28, 2018-11-09 to 2018-11-13, 2019-02-24 to
  2019-03-04.
- The changelog never announces the hitbox fields of the agent table
  (open question 3).

## Generations

|     | Period                                       | Recognized by                                                                                      | Events                                                                                                                                                                                                                                                                                                       |
| --- | -------------------------------------------- | -------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| G0  | 2016-12-08 to 2017-02-13                     | build below 20170214; inside G0 only the build date separates the layouts, none of them documented | layout still moving: skill table added 2016-12-12, `is_adjust` byte dropped 2016-12-21, `is_crit` turned into `result` 2017-02-09, agents re-keyed 2017-02-10 and 2017-02-14. The arcdps author declares logs older than 2017-02-10 unreadable, and the entry of 2017-02-14 asks for builds newer than feb14 |
| G1  | 2017-02-14 to 2018-10-01                     | `header[12] == 0`, build from 20170214                                                             | revision 0 struct, 16-bit skill id and overstack, no `dst_master_instid`, nine bytes of internal tracking                                                                                                                                                                                                    |
| G2  | 2018-10-02 to the last build before 20260501 | `header[12] == 1`, build below 20260501                                                            | revision 1 struct. Strikes, buff ticks, buff applications, buff removals and casts all have `is_statechange == 0` and are told apart by `is_activation`, `is_buffremove`, `buff`, `value` and `buff_dmg`                                                                                                     |
| G3  | build 20260501 on                            | `header[12] == 1`, build from 20260501                                                             | revision 1 struct, every event typed by `is_statechange`; 0 is a strike or a buff tick only                                                                                                                                                                                                                  |

20260501 is the build the README gives for the retirement of
`APIDELAYED`, which the changelog ties to the G3 typing (see [Retired
kinds](#retired-kinds)); the first release with the G3 typing is
2026-05-07. Kinds 67 to 72 in a log are a safer sign of G3 than the build
(open question 9). Measured, the split is clean: no log of a build up to
20260416 holds a kind from 67 to 72, no log from 20260507 on holds an
event typed the G2 way, and no log is dated between the two.

Revision 1 existed from 2018-07-10 behind an ini key (`new_cbtevent`), so
a log of 2018-07 to 2018-09 can carry either revision byte, and the
published writer still sets the byte from that key. Whether the opt-in
struct already had the final layout is open question 10. The byte only
selects the struct: the field rules dated 2018-10-02 follow the build, as
in [Build thresholds](#build-thresholds).

The sizes of the header (16 bytes), of an agent row (96) and of a skill
row (68) never changed after G0. What moved inside the agent row is dated
under [Agent table](#agent-table-96-bytes-per-agent).

## File layout

The header, a `uint32` count and the agent rows, a `uint32` count and the
skill rows, then the events to the end of the file, without a count. The
published writer produces the whole file once the log is over.

### Header, 16 bytes

| Offset | Size | Field                                |
| ------ | ---- | ------------------------------------ |
| 0      | 4    | `EVTC`                               |
| 4      | 8    | build date, `yyyymmdd`               |
| 12     | 1    | revision of the event struct, 0 or 1 |
| 13     | 2    | species id of the log target         |
| 15     | 1    | unused                               |

- Builds before 2016-12-09 zeroed the wrong header bytes (G0).
- The species id was left out from a change of the log stop rules,
  probably the release of 2017-03-31, until 2017-04-13.
- Species id 1: the main page of 2017-02 says manual logs use it, and
  manual logging was removed on 2017-08-25. 2018-10-02 adds log triggers
  for gadgets and for all players in combat. The README of 2022-05 calls
  1 a generic log, started by a squad member entering combat instead of a
  boss. From the README of 2022-11 on, 1 is a WvW log and 2 a map log.
  WvW logging dates from 2019-03-05, map logging from 2022-02-28.
  2023-02-02 fixes logs wrongly typed WvW.
- The target: gadget ids allowed from 2019-09-18, the lowest visible
  species id of the encounter from 2019-12-25, minions allowed from
  2024-06-04. The release of 2025-08-06 could fail to set it, fixed
  2025-08-09.
- 2022-11-11: a log that starts generic and then meets a boss writes
  `LOGNPCUPDATE` (47). The file is written once the log is over, so the
  header presumably holds the final species id; not checked on a log. The
  rules that set the boss changed on 2022-11-29, 2022-12-13, 2022-12-23
  and 2023-01-10, and 2023-01-11 maybe fixes a wrong boss set by the
  release before.

### Agent table, 96 bytes per agent

| Offset | Size | Field                                       | History                                                                                                                                                                                                                                                                                                                                                           |
| ------ | ---- | ------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0      | 8    | `addr`                                      | called `iid` since 2025-08; see `IIDCHANGE` (64)                                                                                                                                                                                                                                                                                                                  |
| 8      | 4    | `prof`                                      | see Gadgets below                                                                                                                                                                                                                                                                                                                                                 |
| 12     | 4    | `is_elite`                                  | 0 or 1 until 2017-09-05, then the elite spec id; `0xFFFFFFFF` for non players                                                                                                                                                                                                                                                                                     |
| 16     | 2    | `toughness`                                 | presumably the low half of an `int32` until 2017-08-19                                                                                                                                                                                                                                                                                                            |
| 18     | 2    | `concentration`                             | added 2017-08-19, always 0 until 2018-07-24                                                                                                                                                                                                                                                                                                                       |
| 20     | 2    | `healing`                                   | presumably the low half of an `int32` until 2017-08-19                                                                                                                                                                                                                                                                                                            |
| 22     | 2    | `pad1`, then `hitbox_width`                 | `pad1` in the `example.cpp` edit of 2017-08-18, still published on 2018-05-14; `hitbox_width` in the writer dated 2022-03-19 (open question 3). Measured: set from build 20230114 on, for few agents in the release of 2024-06-13                                                                                                                                 |
| 24     | 2    | `condition`                                 | presumably the low half of an `int32` until 2017-08-19                                                                                                                                                                                                                                                                                                            |
| 26     | 2    | `pad2`, then `hitbox_height`, then `defunc` | `defunc` in the README of 2026-06, released 2026-07-01: the author says it never was a height. Measured: 0 for two NPCs out of three in build 20230114, set for players and NPCs from 20240613 to 20251115, 0 for them from 20251118, whose entry only says the game build changed, and 0 for every agent from 20260602, a month before the release that drops it |
| 28     | 64   | `name`                                      | `character\0account\0subgroup\0` for players                                                                                                                                                                                                                                                                                                                      |
| 92     | 4    | struct padding                              |                                                                                                                                                                                                                                                                                                                                                                   |

Until 2017-08-19 the twelve bytes at offset 16 were an `int32[3]`, whose
order no source gives. If it was toughness, healing, condition, a small
old value reads the same through the `int16[6]` that replaced it:
`concentration` sits on the upper half of the first value and the two
pads on the upper halves of the others. The stats are `int16_t` in the
code of 2017, `uint16_t` in the writer captured in 2022 and `int16_t`
again in the live file. No README shows the struct before 2026-05.

- Stats are relative to the squad, and arcdps calls them role hints: a 1
  to 10 scale from 2017-02-09, 0 to 10 in the `example.cpp` of 2017,
  which scales every row, NPCs included. The writer captured in 2022
  scales players only, to 0 or 10, and as written gives 0, not 10, above
  0.66 of the squad maximum. It leaves NPC stats raw, capped to `int16`
  since 2018-10-02. When the scaling changed is unknown. 2017-05-23 and
  2020-04-28 fix the hints. WvW logs had none from 2019-03-05 to
  2019-03-29.
- Account name in the name block since 2017-02-14, subgroup since
  2017-02-16. 2017-09-14 fixes guild name and subgroups missing after a
  secondary log start, 2017-09-23 account names broken the day before,
  2023-11-13 subgroups, broken since a build the entry does not name.
  Since 2019-03-29 only squad members get an account and a subgroup.
  2022-11-29 maybe fixes a boss named by a temporary name.
- Gadgets: from 2017-05-04, `is_elite == 0xFFFFFFFF` with the upper half
  of `prof` at `0xFFFF` is a gadget, and the lower half is a pseudo id;
  with another upper half it is an NPC and the lower half is the species
  id. 2017-05-05 fixes `prof` corrupted by that change. No source says
  how to tell them apart before 2017-05-04 (open question 14). The pseudo
  id generation changed on 2017-05-05 and 2018-08-08, and 2026-09-15
  stops it reading uninitialized data. Duplicate gadget rows fixed
  2017-02-10 and maybe 2017-05-04; most gadgets were missing from the
  table until 2019-02-01. Owners: engineer turrets from 2020-05-06,
  banners from 2021-04-29, rocket and sylvari turrets from 2022-07-19,
  WvW siege detached from its owner on 2026-07-01.
- Which agents get a row: every README warns that agents without a name
  or without combat may appear in events and not in the table. An
  internal cap on the number of agents was removed on 2019-12-25. WvW
  logs drop irrelevant agents since 2019-03-05. Players get a row only
  when in combat since 2023-07-18. Since 2024-06-12 instance logs hold
  every initialized agent, other logs only those that interact with the
  squad (tightened to damage interaction on 2025-04-20, partly reverted
  2025-04-28). 2024-06-14 restores hitbox widths lost in the release
  before (measured on the 78 logs dated 20240613: no width for 90% of the
  NPCs and 28% of the gadgets, where no other build passes 10% and 6%),
  2024-06-18 possibly fixes squad members missing from the table.
- 2023-11-10: agents linger after their despawn, so that late events keep
  their agent.
- 2026-08-11: the address of a player becomes a pseudo account value
  instead of an incrementing id (G3).

### Skill table, 68 bytes per skill

`int32 id`, then `char name[64]`. Embedded in the file since 2016-12-12.

- 2017-11-22 fixes skill ids not set, which gave several rows with id 0.
- Skills only seen in `BUFFINITIAL` events were missing until 2022-06-29.
- 2022-08-23: the names of weapon stow and weapon draw were swapped until
  then; the ids did not move.
- 2023-03-01: the skill ids of `EXTENSIONCOMBAT` events join the table.
- 2024-04-21 adds placeholder skills for strikes with skill id 0 (a
  finisher, for one). The entry of 2026-02-03 still calls such strikes
  ignored until then and logs them under 23301; the two entries do not
  agree on what the builds in between wrote.
- 2026-07-02 fixes a missing skill list (G3). Measured on the 37 logs of
  build 20260701: the table has one row, skill 0 named `0`, and the log
  has no `BUFFINFO`, `BUFFFORMULA` or `SKILLTIMING` event, a single
  `SKILLINFO`, for skill 0, and no skill GUID.
- Measured: up to build 20260604 every skill an event names has a row,
  two garbage ids in four logs of build 20250925 aside. From 20260702 the
  custom skills of arcdps (23275 and up) have none.

### Event, revision 0, 64 bytes

| Offset | Size | Field                                                        |
| ------ | ---- | ------------------------------------------------------------ |
| 0      | 8    | `time`                                                       |
| 8      | 8    | `src_agent`                                                  |
| 16     | 8    | `dst_agent`                                                  |
| 24     | 4    | `value`                                                      |
| 28     | 4    | `buff_dmg`                                                   |
| 32     | 2    | `overstack_value`                                            |
| 34     | 2    | `skillid`                                                    |
| 36     | 2    | `src_instid`                                                 |
| 38     | 2    | `dst_instid`                                                 |
| 40     | 2    | `src_master_instid`                                          |
| 42     | 9    | internal tracking (`iss_offset` to `skar_use_alt`), garbage  |
| 51     | 1    | `iff`                                                        |
| 52     | 1    | `buff`                                                       |
| 53     | 1    | `result`                                                     |
| 54     | 1    | `is_activation`                                              |
| 55     | 1    | `is_buffremove`                                              |
| 56     | 1    | `is_ninety`                                                  |
| 57     | 1    | `is_fifty`                                                   |
| 58     | 1    | `is_moving`                                                  |
| 59     | 1    | `is_statechange`                                             |
| 60     | 1    | `is_flanking`, since 2017-02-16                              |
| 61     | 1    | `is_shields`, since 2017-08-11                               |
| 62     | 1    | `result_local` (garbage), then `is_offcycle` from 2018-07-10 |
| 63     | 1    | `ident_local` (garbage), later `pad64`                       |

The changelog counts bytes from 1: its "byte 61" is offset 60, and
`pad61` to `pad64` of revision 1 are offsets 60 to 63. `overstack_value`
became unsigned on 2017-02-10. There is no `dst_master_instid`, and no
room for a trackable id, the per stack id of 2018-12-14: revision 0 logs
cannot match a removal to an application.

### Event, revision 1, 64 bytes

The layout `parse.go` reads. Optional from 2018-07-10, the default from
2018-10-02; whether the optional struct already had this layout is open
question 10.

| Offset | Size | Field                               |
| ------ | ---- | ----------------------------------- |
| 0      | 32   | `time` to `buff_dmg`, as revision 0 |
| 32     | 4    | `overstack_value`, `uint32`         |
| 36     | 4    | `skillid`, `uint32`                 |
| 40     | 2    | `src_instid`                        |
| 42     | 2    | `dst_instid`                        |
| 44     | 2    | `src_master_instid`                 |
| 46     | 2    | `dst_master_instid`                 |
| 48     | 1    | `iff`                               |
| 49     | 1    | `buff`                              |
| 50     | 1    | `result`                            |
| 51     | 1    | `is_activation`                     |
| 52     | 1    | `is_buffremove`                     |
| 53     | 1    | `is_ninety`                         |
| 54     | 1    | `is_fifty`                          |
| 55     | 1    | `is_moving`                         |
| 56     | 1    | `is_statechange`                    |
| 57     | 1    | `is_flanking`                       |
| 58     | 1    | `is_shields`                        |
| 59     | 1    | `is_offcycle`                       |
| 60     | 4    | `pad61` to `pad64`                  |

The state changes of the revision 0 era (kinds 1 to 21) keep their
payload in the first 32 bytes, which both revisions share, position,
velocity and facing included. The exception is `BUFFINITIAL` (18), a full
buff application.

2022-06-28 fixes skill ids still read as 16 bits in some cases. Out of
range skill ids on some events are fixed on 2021-09-23 and possibly again
on 2023-12-16; 2021-12-14 patches a skill id error of the release before.

### Time

`time` is the millisecond clock of the client (`timeGetTime`). Until
2022-02-28 it was taken when arcdps processed the event, since then when
the event was created. G3 logs are only nearly sorted and nothing in the
sources says older ones do better.

### File names and compression

`.evtc` raw. Optional zip through PowerShell from 2017-02-15, named
`.evtc.zip`; renamed `.zevtc` on 2018-12-11, still a zip with one entry.
Compression is built in and always on since 2021-10-19.

## Event typing before 20260501

In G1 and G2 an event with `is_statechange == 0` is typed by its other
fields. The README gives the order of the tests; the last column is the
kind a G3 log uses for the same fact.

| Test, in order                                                                  | Meaning                      | G3 kind                   |
| ------------------------------------------------------------------------------- | ---------------------------- | ------------------------- |
| `is_statechange != 0`                                                           | state change                 | same value                |
| `is_activation` 1 or 2                                                          | cast start                   | `ANIMATIONSTART` (67)     |
| `is_activation` 3 to 6                                                          | cast end                     | `ANIMATIONSTOP` (68)      |
| `is_buffremove` 1                                                               | all stacks of a buff removed | `BUFFREMOVE_ALL` (72)     |
| `is_buffremove` 2 or 3                                                          | one stack removed            | `BUFFREMOVE_SINGLE` (71)  |
| `buff != 0`, `buff_dmg == 0`, `value != 0`, `is_offcycle != 0`, from 2018-12-14 | duration of a stack changed  | `BUFFCHANGE` (70)         |
| `buff != 0`, `buff_dmg == 0`, `value != 0`                                      | buff applied                 | `BUFFAPPLY` (69)          |
| `buff != 0`, `value == 0`                                                       | buff tick                    | `COMBAT` (0), `buff != 0` |
| `buff == 0`                                                                     | strike                       | `COMBAT` (0), `buff == 0` |

The `is_offcycle` test must be skipped before 2018-12-14: that byte is
garbage before 2018-07-10, then set on buff ticks, on strikes too from
2018-10-02, and on applications from 2018-12-14. No README describes an
event with `buff != 0` and both `value` and `buff_dmg` non-zero, and no
log measured holds one. A duration change of 0 ms has `value == 0` and
types as a tick by these rules; the stack id in its pads gives it away.
Build 20230114 holds a few, no later build does. `BUFFINITIAL` (18) has
the fields of a buff application in every generation.

### Cast start

| Field                                               | History                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| --------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `is_activation`                                     | 1 normal, 2 under quickness. 2 is unused since 2019-11-07 (the README says nov 5 2019) and 1 is renamed `START`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `value`                                             | expected duration since 2017-04-04 (before, the expected time sat in `buff_dmg` of the end event). Reworded over time: all significant effects done (README of 2022-05), first significant effect (2025-05), minimum of last trigger point and tooltip time (G3)                                                                                                                                                                                                                                                                                                                                                                             |
| `buff_dmg`                                          | time at which control returns to the agent, since 2019-11-07                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `dst_agent`, `overstack_value`                      | by the README, x/y as two floats and z of the target location of the skill, from 2022-06-28 to the end of G2; not an agent. G3 puts the target agent in `dst_agent` and drops the floats, which the author then says were not a location. Measured on every build from 20230114 to 20260416: `dst_agent` is 0 on 11 to 30% of the cast starts, else a value that looks like a pointer (`0x000001dc30ea3066`), never the address of an agent of the table and not two floats of a position. In G3 it is an agent of the table on 81 to 86% of the cast starts and 0 on the others. What `dst_agent` held before 2022-06-28 is open question 7 |
| `is_ninety`, `is_fifty`, `is_moving`, `is_flanking` | set on cast events in G1 and G2, dropped in G3                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `skillid`                                           | dodges are skill 65001 from 2017-02-13, 23275 from 2022-03-04                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |

Gadget interaction and emotes become cast events on 2026-04-14, three
weeks before G3. 2020-04-28 filters duplicate casts without a source.

### Cast end

| Field           | History                                                                                                                                                                                                                                                            |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `is_activation` | 3 `CANCEL_FIRE`: stopped after the tooltip time. 4 `CANCEL_CANCEL`: stopped before. 5 `RESET`: animation completed, added 2017-04-03. 6 no expected duration known, added 2026-04-14. G3 keeps the values under new names (`MINIMUM`, `CANCEL`, `RESET`, `NODATA`) |
| `value`         | time spent in the animation, on every skill since 2017-02-05 (before, resurrection only)                                                                                                                                                                           |
| `buff_dmg`      | time spent scaled as if no speed change applied, on reset too, since 2019-11-07                                                                                                                                                                                    |
| `skillid`       | 0 on `RESET` until 2017-09-30, then the skill that ended; the dodge id on the reset that follows a dodge, since 2017-10-10                                                                                                                                         |

A dodge is followed by a `RESET` since 2017-09-13. Detection changed
often: a second cancel detection on 2017-07-31 (some skill transitions
still without an end), cancels not detected fixed 2018-12-11, a rework on
2021-09-23 with fixes on 2021-09-27 and 2021-09-28, missing ends of
second-use skills and weapon stow added 2022-06-28, stop reasons adjusted
2026-04-22. The release of 2021-05-29 wrote 0 in the times of cancel
events, fixed 2021-05-30.

The G3 README describes `value` as the time scaled for speed and
`buff_dmg` as the time not scaled. Measured, it is the G2 pair and not
its swap: `value` is the smaller of the two on two cast ends out of three
in G2 and three out of four in G3, the larger on one out of six and one
out of five, and no build stands out on either side of 20260501.
`is_activation` 6 appears in build 20260414, as announced.

### Buff removal

`src_agent` lost the buff, `dst_agent` removed it, the reverse of an
application. Present since 2016-12-12.

| Field              | History                                                                                                                                                                                                 |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `is_buffremove`    | 1 all, 2 single, 3 manual: a single removal arcdps derives from a remove-all or from leaving combat, to be ignored when counting strips and cleanses. Derived removals are logged as 3 since 2017-05-18 |
| `value`            | duration removed                                                                                                                                                                                        |
| `buff_dmg`         | duration removed counted as intensity; the longest stack for intensity buffs since 2017-10-10. Can overflow on a remove-all                                                                             |
| `result`           | number of stacks removed (README of 2022-05), remove-all only from the README of 2022-11                                                                                                                |
| `pad61` to `pad64` | trackable id, single removals only, since 2018-12-14. The G2 README calls it the buff instance id                                                                                                       |
| `is_shields`       | 1 when the removed stack was active, single removals, since 2021-01-02                                                                                                                                  |

Single removals sent by the server: the README of 2017-10 says they are
disabled and that a cleanse writes one event per stack. They are logged
for stability from 2018-08-08 and for every buff from 2018-12-14. The
removal of the last stability stack was dropped from 2017-04-30 to
2018-08-08.

2017-04-17 removes spurious application and removal events. 2018-06-28
maybe fixes removals reporting 0 for duration and intensity. 2021-01-02:
for stability-like buffs the remaining duration is counted as intensity.
2021-02-23: a removal counts as a squad event when the buff came from a
squad member.

### Buff application and duration change

| Field              | History                                                                                                                                                                                                                                                                                                                       |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `value`            | duration applied; the duration difference on a change                                                                                                                                                                                                                                                                         |
| `overstack_value`  | duration of the stack this one is expected to replace; the new duration on a change. 16 bits in revision 0, 65.5 s at most: the changelog of 2018-07-10 blames it for wrong numbers on long buffs reapplied at their cap. Unsigned since 2017-02-10, and the author advises to ignore it on non combat buffs before that date |
| `is_offcycle`      | 0 application, non-zero duration change, since 2018-12-14                                                                                                                                                                                                                                                                     |
| `pad61` to `pad64` | trackable id since 2018-12-14                                                                                                                                                                                                                                                                                                 |
| `is_shields`       | stack applied as active, since 2019-01-08; 2 for stability-like buffs since 2021-04-29. 2024-06-09 fixes a wrong initial value on some buffs                                                                                                                                                                                  |

The overstack is an estimate: taken from the shortest remaining stack
from 2017-02-04, broken for an unknown span before 2018-10-10, and
counting the stack being added, after a game update, until 2020-04-28.

Duration changes had source and destination swapped in the release of
2019-01-08, fixed the next day. They carried wrong values until
2023-09-02, and 2023-11-07 possibly fixes `value` again (the difference
to the old duration, as the README said since 2022-05). A stack applied
as active gets no `BUFFACTIVE` (27) event (2019-01-08). The README notes
that from 2021-04-29 applications are delayed to the end of combat unless
both agents are in the squad; whether that moves them in the file is open
question 8.

`BUFFINITIAL`: duration 0 from 2018-04-24, remaining duration from
2018-10-02, original duration in `buff_dmg` from 2023-11-07.

### Buff tick

| Field         | History                                                                                                         |
| ------------- | --------------------------------------------------------------------------------------------------------------- |
| `buff_dmg`    | simulated damage. Always 0 on a resisted tick since 2018-06-26                                                  |
| `value`       | 0. The README of 2017-10 reads `value == 0 && buff_dmg == 0` as a tick negated by invulnerability or resistance |
| `is_offcycle` | 0 on the tick timer, non-zero otherwise, since 2018-07-10; of enum `cbtbuffcycle` since 2021-05-11              |
| `result`      | 0 expected to hit, 1 invulnerable through a buff, 2 to 4 through a player skill (README of 2022-05)             |
| `pad61`       | 1 when the target is downed, since 2021-08-28                                                                   |
| `is_shields`  | barrier flag, fixed 2018-07-25                                                                                  |

One event per stack per target per tick from 2016-12-21; ticks are
coalesced since 2021-05-25, so older logs hold more and smaller ticks.
Ticks of 0 damage were maybe not logged before 2018-06-26, and negated
ones sometimes not before 2018-07-25. The damage is a simulation whose
formulas the changelog adjusts dozens of times (confusion, torment, tick
timing, siphons); those lines are in `changelog_all.tsv` and change
values, not the encoding. G3 moves the cycle into `result` (14 to 18) and
the downed flag into `is_offcycle`. Measured: a G2 tick has `result` 0, 1
or 3, never 2 or 4, and a cycle of 0, 3, 4 or 5, never 1 or 2. A G3 tick
has `result` 14, 16, 17 or 18, never 15, and a negated one has 6
(`ABSORB`): about 1.5% of the ticks, as `result` 1 in G2. A few carry 13
(`INVERT`).

### Strike

| Field             | History                                                                                                                                                                                                                      |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `result`          | see [cbtresult](#cbtresult). Called `is_crit` before 2017-02-09                                                                                                                                                              |
| `value`           | damage, barrier included. The README of 2017-10 reserves negative values for healing, which it does not log yet                                                                                                              |
| `overstack_value` | barrier part of the damage since 2018-10-02                                                                                                                                                                                  |
| `is_shields`      | barrier absorbed some of it, since 2017-08-11                                                                                                                                                                                |
| `is_offcycle`     | target downed, since 2018-10-02                                                                                                                                                                                              |
| `is_flanking`     | since 2017-02-16. The arc was wrong until 2017-12-25; computed from the line between the two agents instead of the facing of the source from 2018-11-25; an angle from 1 to 135 degrees since 2022-12-13, 135 being the rear |
| `is_moving`       | source moving. Bit 1 is the target since 2021-05-11 but was never set before 2024-07-16                                                                                                                                      |
| `iff`             | from content affinity for characters since 2018-10-02, a guess for gadgets; unknown (2) when source or destination is 0, since 2018-12-14                                                                                    |

Until 2018-05-22 a strike on a gadget could carry the out of range result
`0x10`. 2017-08-12 fixes glance set on a killing strike. Which strikes
are written also moved: glance, block, evade and the like from
2017-02-09, none against level 1 targets from 2018-10-03, damaging events
outside combat too from 2018-12-11, without the check that they happen
near the squad in instances from 2020-12-01 and in every mode from
2021-09-22. 2020-06-23 fixes an agent rarely missing all its strikes,
2020-08-15 minion data missing for some builds, 2024-06-19 events with
zero source and destination wrongly filtered. Since 2025-03-13 an event
whose agent lookup failed keeps the instance id with a zero address.

## Enums

`cbtstatechange`, `cbtresult`, `cbtactivation`, `cbtbuffremove`, `iff`
and `gwlanguage` only ever grew at their end, with renames
(`enums_matrix.txt`). The constants of `state.go` and `enums.go` read old
logs as they are; what changes is which values can occur and what the
other fields hold. Three things did move: the custom skill ids
(2022-03-04), the formula attributes, which lose `ATTR_CUST_FLATINC` on
2022-03-06 (open question 6), and `cbtbuffcycle`, which G3 merges into
`cbtresult`. The content types did not, but the README numbers them one
short from 4 on (see [Others](#others)).

The changelog sometimes uses a new name before a captured README does.

### cbtstatechange

"Released" is the changelog date. A bracket means no entry announces the
kind: the bounds come from the neighbours or from the first README that
shows it. Names are the current ones, without `CBTS_`. "Until the README
of X" means X is the first to show the name that replaced it. An empty
"Released" cell means the value is as old as the sources. "Reset" is the
word of the changelog; it presumably clears the last sampled value, so
that the current one is written again.

| Value    | Name                                                                                 | Earlier names                                                                                                   | Released                                      | Payload history                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| -------- | ------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------- | --------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 0        | `COMBAT`                                                                             | `NONE` until the README of 2026-05                                                                              |                                               | see [Event typing](#event-typing-before-20260501)                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| 1        | `ENTERCOMBAT`                                                                        |                                                                                                                 | [by 2017-02-09]                               | 2017-02-15 `value` is the max health of an NPC, moved to `dst_agent` on 2017-02-17; `dst_agent` is the subgroup from 2017-07-02, and no source says what an NPC carries there afterwards; `value` profession and `buff_dmg` elite spec from 2024-06-12. 2024-11-20 fixes a stray event                                                                                                                                                                                                                       |
| 2        | `EXITCOMBAT`                                                                         |                                                                                                                 | [by 2017-02-09]                               | subgroup and elite spec from 2025-06-03; the profession is in the README of 2026-05                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 3 to 5   | `CHANGEUP`, `CHANGEDEAD`, `CHANGEDOWN`                                               |                                                                                                                 | [by 2017-02-09]                               | the release of 2024-06-12 swapped dead and down, fixed the next day                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 6, 7     | `SPAWN`, `DESPAWN`                                                                   |                                                                                                                 | 2017-02-09                                    | 2025-08-29 possibly fixes missing spawns of players; none outside the instance map in map logs since 2025-09-23                                                                                                                                                                                                                                                                                                                                                                                              |
| 8        | `HEALTHPCTUPDATE`                                                                    | `HEALTHUPDATE` until the README of 2024-07                                                                      | 2017-02-15                                    | boss only, 0.5% steps, in `value`; `dst_agent` from 2017-02-17; never 0 since 2017-08-15; reset for all agents at log start from 2019-04-23 and at each spawn from 2023-09-12; needless events on gadgets without health fixed 2026-07-01. Measured: such an event holds `0x8000000000000000` in `dst_agent`, what a division by zero leaves; 16,899 health updates of build 20240613 and 50 of 20240709                                                                                                     |
| 9        | `SQCOMBATSTART`                                                                      | `LOGSTART` until the README of 2024-07                                                                          | 2017-02-16                                    | 2017-07-11: `src_agent` a constant (`0x637261`), `value` server unix time, `buff_dmg` local unix time. 2018-12-12 "fixes" a log start left uninitialized (the quotes are the entry's). The README of 2023-10 drops the constant. In map logs since 2025-03-15. The changelog of 2025-08-06 announces `dst_agent` 2 for a map log and 3 for a boss log, valid for earlier map logs too; no captured README has it, but the logs measured agree: 3 on every boss log from build 20230114 on, 2 on the map logs |
| 10       | `SQCOMBATEND`                                                                        | `LOGEND` until the README of 2025-05                                                                            | 2017-02-16                                    | could be missing before 2019-10-17, possibly fixed again 2020-05-06. `dst_agent` bit 0 when the POV left the map, from 2025-08-29, in map logs from 2025-09-08. Measured: 0 in every log up to build 20250828, 0 or 1 from 20250829, never the 2 or 3 of kind 9                                                                                                                                                                                                                                              |
| 11       | `WEAPSWAP`                                                                           |                                                                                                                 | [2017-02-16 to 2017-03-27] (open question 12) | `dst_agent` new set (0/1 water, 4/5 land); `value` old set from 2024-06-27                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 12       | `MAXHEALTHUPDATE`                                                                    |                                                                                                                 | [2017-02-16 to 2017-03-27]                    | `dst_agent` new maximum; reset like kind 8; non players only by the README of 2024-07; a maximum of 1 on agents without health fixed 2026-07-08                                                                                                                                                                                                                                                                                                                                                              |
| 13       | `POINTOFVIEW`                                                                        |                                                                                                                 | 2017-03-27                                    | 2017-06-13 fixes the agent it points to                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 14       | `LANGUAGE`                                                                           |                                                                                                                 | 2017-04-15                                    | broken by the release of 2023-07-18, fixed the next day                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 15       | `GWBUILD`                                                                            |                                                                                                                 | 2017-05-14                                    |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 16       | `SHARDID`                                                                            |                                                                                                                 | [2017-05-14 to 2017-09-05]                    | `dst_agent` upper 16 bits of the shard from 2026-02-26                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 17       | `REWARD`                                                                             |                                                                                                                 | 2017-09-05                                    | `dst_agent` reward id, `value` reward type                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 18       | `BUFFINITIAL`                                                                        |                                                                                                                 | 2018-04-24                                    | see [Buff application](#buff-application-and-duration-change). `buff` is 18 as well in the READMEs up to 2024-05. Measured: from build 20250708 to 20250913 four events out of five are empty (`skillid` 0, no source, no stack id, `value` 1), about 2,500 a log, all in its first two seconds. They are gone in 20250923, and no changelog entry mentions them                                                                                                                                             |
| 19, 20   | `POSITION`, `VELOCITY`                                                               |                                                                                                                 | 2018-06-26                                    | players and boss every 300 ms; raid adds from 2018-07-24; a gadget gets a position and a facing when it joins the table, from 2018-08-08; reset for all agents at log start from 2019-04-08 and at each spawn from 2023-09-12; first samples could be missing before 2019-08-27; every agent of the table from 2024-06-12; outside combat in map logs from 2025-07-29; garbage events maybe fixed 2026-09-15                                                                                                 |
| 21       | `FACING`                                                                             |                                                                                                                 | 2018-07-18                                    | every 500 ms, same history as 19 and 20                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 22       | `TEAMCHANGE`                                                                         |                                                                                                                 | 2018-10-02                                    | hostile targets from the release of 2021-05-14 (announced 2021-05-25); enemies missing until 2024-04-24. From 2024-06-12 written at despawn, not spawn, with the old team in `value`; untagged events at the end of the log fixed 2024-06-13; 2025-09-13 fixes team and spec again                                                                                                                                                                                                                           |
| 23       | `ATTACKTARGET`                                                                       |                                                                                                                 | 2018-10-02                                    | `value` was the targetable state in the README of 2022-05, gone in 2024-07                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 24       | `TARGETABLE`                                                                         |                                                                                                                 | 2018-10-02                                    | missing for attack targets before 2020-03-30; characters and players too from 2024-06-12 (stealth); 0, 1 or 2 with new rules from 2026-06-02                                                                                                                                                                                                                                                                                                                                                                 |
| 25       | `MAPID`                                                                              |                                                                                                                 | 2018-10-08                                    | the changelog says 26, the README 25. `dst_agent` map type from 2026-04-14                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 26       | `REPLINFO`                                                                           |                                                                                                                 | 2018-12-11                                    | internal                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 27       | `BUFFACTIVE`                                                                         | `STACKACTIVE` until the README of 2026-05                                                                       | 2018-12-14                                    | `dst_agent` trackable id; `skillid` the buff from 2019-09-18; `value` current duration in the README of 2024-07                                                                                                                                                                                                                                                                                                                                                                                              |
| 28       | `BUFFDEACTIVE`                                                                       | `STACKRESET` until the README of 2026-05                                                                        | 2019-01-08                                    | `value` duration to reset to, `pad61` to `pad64` trackable id                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| 29       | `GUILD`                                                                              |                                                                                                                 | 2019-03-29                                    | 16 bytes from `dst_agent`, in client byte order, to rearrange for the API form (README of 2022-05); 2025-10-28 fixes the GUID                                                                                                                                                                                                                                                                                                                                                                                |
| 30       | `BUFFINFO`                                                                           |                                                                                                                 | 2019-12-25                                    | stacking type missing before 2020-04-28, which adds `pad62`; `overstack_value` duration cap from 2021-05-11; `pad63` from 2025-07-29                                                                                                                                                                                                                                                                                                                                                                         |
| 31       | `BUFFFORMULA`                                                                        |                                                                                                                 | 2019-12-25                                    | more data 2020-05-06; buff conditions at `src_instid` from 2021-02-23; `ATTR_CUST_FLATINC` deleted by the release of 2022-03-06 (announced 2022-03-08), so the ids after it, if any, shift (open question 6); ninth float (content reference) in the README of 2024-07; original attribute instead of the remapped one from 2025-09-13                                                                                                                                                                       |
| 32       | `SKILLINFO`                                                                          |                                                                                                                 | 2019-12-25                                    | first float called recharge in the README of 2022-05, cost in 2024-07                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| 33       | `SKILLTIMING`                                                                        |                                                                                                                 | 2019-12-25                                    | the changelog calls it skillformula                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 34, 35   | `DEFIANCEBARSTATE`, `DEFIANCEBARPERCENT`                                             | `BREAKBARSTATE`, `BREAKBARPERCENT` until the README of 2026-05; the changelog says defiance bar from 2026-02-03 | 2020-05-06                                    | state in `value` by the README of 2022-05, in `dst_agent` by the READMEs from 2024-07. No changelog entry moves it. Measured: in `value` in every build from 20230114 to 20260915, with `dst_agent` always 0 (open question 13). Gadgets fixed 2026-02-03                                                                                                                                                                                                                                                    |
| 36       | `INTEGRITY`                                                                          | `ERROR` until the README of 2024-07                                                                             | 2020-05-13                                    | run-on boss warning from 2024-06-12; one per boss, with a `g` after a gadget id, from 2026-02-26                                                                                                                                                                                                                                                                                                                                                                                                             |
| 37       | `MARKER`                                                                             | `TAG` until the README of 2024-05                                                                               | 2020-06-09                                    | players likely to be commanders, written when the agent enters combat, at squad combat start from 2022-08-23. `buff` 1 for a commander tag from 2022-08-23. Additions and removals (id 0) from 2024-03-28. From 2024-04-18 every change writes a 0 then one event per marker. Missing events fixed 2020-07-18 and 2022-09-14; event spam fixed 2024-03-30; markers broken by the release of 2024-12-10 fixed the next day                                                                                    |
| 38       | `BARRIERPCTUPDATE`                                                                   | `BARRIERUPDATE` until the README of 2024-07                                                                     | 2020-12-01                                    | needless events on gadgets without health fixed 2026-07-01. Measured: until build 20260604, 96% of the barrier updates are of that sort and hold `0x8000000000000000`, which is 19% of all the events of the logs; none from 20260701                                                                                                                                                                                                                                                                        |
| 39       | `STATRESET_DEFUNC`                                                                   | `STATRESET` until the README of 2026-05                                                                         | 2021-03-29                                    | not in the file by the READMEs up to 2024-05, in the file by those from 2024-07; `src_agent` boss species id from 2021-06-19                                                                                                                                                                                                                                                                                                                                                                                 |
| 40       | `EXTENSION`                                                                          |                                                                                                                 | 2021-04-29                                    | signature in `pad61` to `pad64`                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 41       | `APIDELAYED_DEFUNC`                                                                  | `APIDELAYED` until the README of 2026-05                                                                        | 2021-04-29                                    | never in a file                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 42       | `INSTANCESTART`                                                                      |                                                                                                                 | 2022-02-28                                    | `value` server socket from 2024-07-09                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| 43       | `RATEHEALTH`                                                                         | `TICKRATE` until the README of 2024-07                                                                          | 2022-05-20                                    | `src_agent` 25 minus the tick rate, every 500 ms while the rate is under 21; replaced by `TICK` (84)                                                                                                                                                                                                                                                                                                                                                                                                         |
| 44       | `LAST90BEFOREDOWN_DEFUNC`                                                            | `LAST90BEFOREDOWN` until the README of 2026-05                                                                  | 2022-05-20                                    | `dst_agent` time since the agent was last at 90% health                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 45       | `EFFECT1_DEFUNC`                                                                     | `EFFECT` until the README of 2026-05                                                                            | 2022-06-28                                    | see [Effects](#effects)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 46       | `IDTOGUID`                                                                           |                                                                                                                 | 2022-06-28                                    | markers from 2022-07-01. To be ignored in logs older than 2022-07-09. Effect GUIDs missing in the release of 2023-07-18. Extra fields per content type from 2024-10-30, durations unscaled from 2024-12-10, missing in map logs until 2025-08-27. For the content types see [Others](#others)                                                                                                                                                                                                                |
| 47       | `LOGNPCUPDATE`                                                                       |                                                                                                                 | 2022-11-11                                    | `dst_agent` and the gadget flag from 2023-01-10                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 48       | `IDLEEVENT`                                                                          |                                                                                                                 | [2022-11-26 to 2023-03-01]                    | internal                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 49       | `EXTENSIONCOMBAT`                                                                    |                                                                                                                 | 2023-03-01                                    |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 50       | `FRACTALSCALE`                                                                       |                                                                                                                 | 2023-07-18                                    |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 51       | `EFFECT2_DEFUNC`                                                                     | `EFFECT2` until the README of 2026-05                                                                           | 2023-07-18                                    | see [Effects](#effects)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 52       | `RULESET`                                                                            |                                                                                                                 | 2024-02-05                                    | written when the recording player wears a ruleset buff: in none of the logs measured                                                                                                                                                                                                                                                                                                                                                                                                                         |
| 53       | `SQUADMARKER_GROUND`                                                                 | `SQUADMARKER` until the README of 2026-08                                                                       | 2024-03-28                                    | initial positions possibly fixed 2024-06-18                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 54       | `ARCBUILD`                                                                           |                                                                                                                 | 2024-06-14                                    |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 55       | `GLIDER`                                                                             |                                                                                                                 | 2024-06-27                                    |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 56       | `STUNBREAK`                                                                          |                                                                                                                 | 2024-07-09                                    |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 57 to 59 | `MISSILECREATE`, `MISSILELAUNCH`, `MISSILEREMOVE`                                    |                                                                                                                 | 2025-05-25                                    | launch radius fixed 2025-06-03; position and target agent on remove from 2026-07-01                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 60 to 63 | `EFFECTGROUNDCREATE`, `EFFECTGROUNDREMOVE`, `EFFECTAGENTCREATE`, `EFFECTAGENTREMOVE` |                                                                                                                 | 2025-06-03                                    | scale override from the start; events without an agent missing until 2025-07-08                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 64       | `IIDCHANGE`                                                                          |                                                                                                                 | 2025-08-19                                    | for the POV too from 2025-08-27; none outside the instance map from 2025-09-23                                                                                                                                                                                                                                                                                                                                                                                                                               |
| 65       | `MAPCHANGE`                                                                          |                                                                                                                 | 2025-08-27                                    | new map type 2026-05-07                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 66       | `EARLYEXIT`                                                                          |                                                                                                                 | [2025-08-27 to 2025-08-29]                    | internal; the changelog only mentions its revert on 2025-08-29                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| 67 to 75 | `ANIMATIONSTART` to `WVWOBJECTIVESTATUS`                                             |                                                                                                                 | 2026-05-07                                    | G3. `WVWOBJECTIVESTATUS` upgrade progress at `pad61` to `pad64` from 2026-06-02; `TRANSFORMATION` gains `value`, `is_shields` and `is_offcycle` in the README of 2026-08; `WVWTEAMS` at the end of the log from 2026-09-15                                                                                                                                                                                                                                                                                   |
| 76 to 87 | `STEALTHCHANGE` to `GADGETMODELINFO`                                                 |                                                                                                                 | 2026-06-02 to 2026-09-15                      | G3. `JUMP` (86) was never written before 2026-09-15. Measured on the 48 logs of build 20260915: jumps in 40, a ping of 16 to 262 in every `TICK` (84), where every earlier build writes 0, `GADGETMODELINFO` (87) in all, and half of the `TELEPORT` events (85) without a target, against 9 out of 68,096 in build 20260816                                                                                                                                                                                 |
| 88       | `FLYTO`                                                                              |                                                                                                                 | 2026-09-20                                    | G3. The README of 2026-09 still omits the kind the changelog announces. Measured on the 17 logs of build 20260920: 1,150 events, `src_agent` always an agent of the table, `dst_agent` the destination as three int16 of the game coordinate divided by ten then a fourth int16 from 20 to 260 (usually 100), `is_flanking` 1 on the start and 0 on the landing about 880 ms later, every other field zero                                                                                                   |

From 2024-06-12 the kinds the README marks "limited to agent table" are
written for every agent of the table in every mode. Before, positions,
velocity and facing were kept for players, the boss and the raid adds
named in the arcdps configuration; the scope of the other kinds is not
documented.

The README runs ahead of the changelog in 2025 and 2026. The capture of
2025-05-23 lists kinds 57 to 59, released two days later. The capture of
2026-06-10 lists kinds 79 to 82, 2026-06-14 kind 83 and 2026-06-30 kind
84, all released on 2026-07-01, with 80 to 82 under other names in the
changelog (`GADGETCAPTURECREATE`, `GADGETCAPTUREPROGRESS`,
`GADGETCAPTUREREMOVE`).

### Retired kinds

The README dates a retirement by build, the changelog by release. Kinds
carry the name they had when retired.

| Kind                    | README, not used since | Changelog  |
| ----------------------- | ---------------------- | ---------- |
| `EFFECT` (45)           | 230716                 | 2023-07-18 |
| `LAST90BEFOREDOWN` (44) | 240529                 | 2024-06-12 |
| `EFFECT2` (51)          | 250526                 | 2025-06-03 |
| `STATRESET` (39)        | 260402                 | 2026-04-14 |
| `APIDELAYED` (41)       | 260501                 | 2026-05-07 |
| `RATEHEALTH` (43)       | 260627                 | 2026-07-01 |

No README gives a build for the end of the G2 typing. The changelog
retires `APIDELAYED` because buffs became state changes, which ties the
two and gives 20260501.

### Effects

Three encodings, one at a time. Measured: kind 45 in build 20230114, 51
from 20240613 to 20240723, 60 to 63 from 20250708, never two of them in
one build.

| Builds                                                | Kind     | Layout                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| ----------------------------------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| release of 2022-06-28 to the last build before 230716 | 45       | `src_agent` owner, `dst_agent` when played on an agent, else `float[3]` position at `value`; orientation as `float[2]` at `iff` and one `float` at `pad61`; `skillid` effect id; `uint16` duration at `is_shields`, or a trackable id when `is_flanking` is set; effect id 0 ends the tracked effect named at `is_shields`                                                                                                                  |
| build 230716 to the last build before 250526          | 51       | agents and position unchanged; `uint32` duration at `iff`; `uint32` trackable id at `is_buffremove`; orientation as `int16[3]` at `is_shields`, the original value times 1000. From the README of 2024-07, an event with no agent and no position ends the effect. No README says where kind 51 keeps the effect id. Measured: `skillid`, since every one of the 851,185 effects of builds 20240613 to 20240723 resolves through `IDTOGUID` |
| build 250526 on                                       | 60 to 63 | the current layout, documented in `state.go`. Measured: the same fields are non-zero from build 20250708 to 20260915 (open question 5)                                                                                                                                                                                                                                                                                                      |

For kind 51: the release of 2023-07-18 used another orientation encoding,
changed the next day. `is_flanking` marks a moving platform from
2023-08-22. The duration was still clamped to 16 bits until 2023-12-13.
Effects on an agent had no trackable id and no end event before
2024-10-30. A zero duration may mean the fixed length given by `IDTOGUID`
(README of 2025-05).

### cbtresult

Names without `CBTR_`.

| Value    | Name                                                              | Earlier names                                                             | Released                           | Notes                                                                                                                                       |
| -------- | ----------------------------------------------------------------- | ------------------------------------------------------------------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| 0 to 2   | `STRIKE_DAMAGENORMAL`, `STRIKE_DAMAGECRIT`, `STRIKE_DAMAGEGLANCE` | `NORMAL`, `CRIT`, `GLANCE` until the README of 2026-05                    | 2017-02-09                         | the byte was `is_crit` before, and glancing strikes were not logged                                                                         |
| 3 to 7   | `BLOCK`, `EVADE`, `INTERRUPT`, `ABSORB`, `BLIND`                  |                                                                           | 2017-02-09                         | such strikes were not logged before                                                                                                         |
| 8        | `KILLINGBLOW`                                                     |                                                                           | 2017-02-09                         | every source from 2018-07-24                                                                                                                |
| 9        | `DOWNED`                                                          |                                                                           | 2018-07-24                         |                                                                                                                                             |
| 10       | `DEFIANCE_DAMAGENORMAL`                                           | `BREAKBAR` until the README of 2026-05                                    | 2020-11-17                         | `value` the defiance damage; against gadgets from 2021-09-22; its skill is the last damaging skill instead of a placeholder from 2021-12-14 |
| 11       | `SKILLCAST`                                                       | `ACTIVATION` until the README of 2026-05, `cbtr_instant` in the changelog | 2021-05-25, announced the next day |                                                                                                                                             |
| 12       | `CROWDCONTROL`                                                    |                                                                           | 2024-06-27                         | from 2024-07-09 `value` is the duration, and the sum of `value` and `overstack_value` the defiance calculation                              |
| 13       | `INVERT`                                                          |                                                                           | [2024-06-27 to 2026-05-07]         | first in the README of 2026-05                                                                                                              |
| 14 to 18 | `BUFF_DAMAGE*`                                                    |                                                                           | 2026-05-07                         | G3, the former `cbtbuffcycle`                                                                                                               |

8 to 12 are not damage. Gadgets give no crit or glance from 2017-05-04;
crit comes back on 2017-09-07.

### cbtactivation, now cbtanimation

Names without `ACTV_`.

| Value | Name               | Earlier names                                                                           | Released   | Notes                   |
| ----- | ------------------ | --------------------------------------------------------------------------------------- | ---------- | ----------------------- |
| 0     | `NONE`             |                                                                                         |            |                         |
| 1     | `START_DEFUNC`     | `NORMAL` until the README of 2022-05, `START` until the README of 2026-05               |            | G1 and G2 only          |
| 2     | `QUICKNESS_DEFUNC` | `QUICKNESS` until the README of 2022-05, `QUICKNESS_UNUSED` until the README of 2026-05 |            | unused since 2019-11-07 |
| 3     | `MINIMUM`          | `CANCEL_FIRE` until the README of 2026-05                                               |            |                         |
| 4     | `CANCEL`           | `CANCEL_CANCEL` until the README of 2026-05                                             |            |                         |
| 5     | `RESET`            |                                                                                         | 2017-04-03 |                         |
| 6     | `NODATA`           | `ACTV_CANCEL_NODATA` in the changelog                                                   | 2026-04-14 |                         |

### cbtbuffremove

0 none, 1 all, 2 single, 3 manual. Unchanged.

### cbtbuffcycle, G2 only

Read from `is_offcycle` of a buff tick. 0 on the tick timer, 1 off the
timer and resistable, 2 off the timer and not resistable, 3 to the target
when hitting it, 4 to the source when hitting the target, 5 to the target
when the source loses a stack. 2 is retired: the README says that before
2021-05 the cases of 3 to 5 were lumped there. The changelog adds the
enum on 2021-05-11 and before that only documents zero or non-zero, from
2018-07-10. In G3 `result` holds 14, 15, 16, 17 and 18 for the old 0, 1,
3, 4 and 5.

### Custom skill ids

| Id             | Meaning                                                                                                                                                                                                                                        |
| -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1066           | resurrect, a game skill without a name, listed by the READMEs up to 2024-07                                                                                                                                                                    |
| 1175           | bandage, personal healing only, listed up to 2024-07                                                                                                                                                                                           |
| 65001          | dodge, until 2022-03-04                                                                                                                                                                                                                        |
| 23275          | dodge, from 2022-03-04                                                                                                                                                                                                                         |
| 23283          | `CSK_BREAKBAR_DEFUNC`. Measured: in no log from build 20230114 on                                                                                                                                                                              |
| 23276 to 23309 | the rest of `n_customskill` as the READMEs from 2026-05 list it. The changelog announces the list on 2026-04-14, in a README version that was never captured; 23304 and 23305 date from 2026-05-07, 23308 and 23309 from the README of 2026-06 |
| 23301          | also takes the strikes with skill id 0, from 2026-02-03                                                                                                                                                                                        |

`CSK_GENERICWATERFLOATSINK` (23298) is split into 23304 and 23305 in G3.

### Others

- `iff`: 0 friend, 1 foe, 2 unknown; history under [Strike](#strike).
- `gwlanguage`: 0 English, 2 French, 3 German, 4 Spanish; 5 Chinese first
  seen in the README of 2022-05.
- `n_contentlocal`: 0 effect and 1 marker (2022), 2 skill and 3 species
  (2025-04-28), whose ids differ between the Chinese client and the
  others. The READMEs from 2026-05 go on with 4 emote and 5
  transformation; the logs do not. Measured: 4 is a team from build
  20260226 to 20260604, as the changelog of 2026-02-26 says (every id is
  a team of a `TEAMCHANGE`); 5 is an emote from 20260414 to 20260604 (in
  G3 every id is in an emote cast); 6 is a transformation from 20260507
  on (every id is the `skillid` of a `TRANSFORMATION`). From build
  20260701 no log holds a 4 or a 5.
- `e_buffcategory` (boon 0, any 1, condition 2, food 4, upgrade 6, boost
  8, trait 11, enhancement 13, stance 16) is in the READMEs of 2022 only.
- `e_attribute` for `BUFFFORMULA` is in the READMEs of 2022-05 to
  2024-07.
- `n_animationstart` and `n_animationstop` (debug only) exist from the
  README of 2026-05.

## Chronology

The releases that changed what a log holds, oldest first. A fix is listed
when it changes data a reader would trust, with the hedge of its entry
(possibly, maybe) kept. A kind or a field carries the name it had at the
time. Left out: the simulation formulas of condition damage, the default
list of boss ids, file and folder naming apart from the extension, the
realtime API unless it shares the change, and the rules that start, stop
and save a log unless they change what a saved log holds. Of those rules,
these decide which logs exist: a minimum of 300 events from 2017-06-26,
750 by the time a check that the boss was engaged replaced it on
2018-12-02; no log without direct damage to the target from 2018-12-25;
at least one damage event with the boss from 2022-11-11.

### 2016, G0

| Date  | Change                                                                                                                                                                                          |
| ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 12-08 | buff adjustment events get their agent ids; players keyed by a unique agent map id instead of the character id; `src_master_cid` links a minion to its player                                   |
| 12-09 | header zeroing fixed                                                                                                                                                                            |
| 12-10 | physical hits on gadgets detected                                                                                                                                                               |
| 12-12 | skill table embedded in the file; buff removal events; end of encounter check restored; stray state change logging removed                                                                      |
| 12-16 | gadgets get a proper destination agent on buff events                                                                                                                                           |
| 12-21 | `is_adjust` byte removed, `is_statechange` likely in its place; buff events split into tick (`buff_dmg`) and application (`value`); one tick event per stack per target; logs about half larger |

### 2017, G0 then G1

| Date         | Change                                                                                                                                                                                                       |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 02-03, 02-04 | overstack taken from the shortest remaining stack, by default from 02-04                                                                                                                                     |
| 02-05        | cast end `value` is the time spent, on every skill                                                                                                                                                           |
| 02-09        | `is_crit` becomes `result`, with 8 for a killing blow; glance, block, evade, interrupt, absorb and blind strikes logged; spawn (6) and despawn (7); stats on a 1 to 10 scale                                 |
| 02-10        | `x_cid` fields renamed `x_instid`, agents keyed by map instance id; boss found through the species id of the header; duplicate gadget rows fixed; `overstack_value` unsigned; older logs declared unreadable |
| 02-13        | dodge as a cast of custom skill 65001; manual logs only in instances                                                                                                                                         |
| 02-14        | agent identification changed again, agents that did not last the whole log were untrackable before; account name in the name block                                                                           |
| 02-15        | optional zip compression; NPC max health in `ENTERCOMBAT`; health updates (8)                                                                                                                                |
| 02-16        | subgroup in the name block; log start (9) and log end (10); `is_flanking`                                                                                                                                    |
| 02-17        | max health and health ratio moved from `value` to `dst_agent`                                                                                                                                                |
| 03-27        | point of view (13); subgroup stale for a few builds, possibly fixed                                                                                                                                          |
| 04-03        | `is_activation` 5, reset                                                                                                                                                                                     |
| 04-04        | expected cast time moves from `buff_dmg` of the end to `value` of the start                                                                                                                                  |
| 04-13        | species id missing from the header, fixed                                                                                                                                                                    |
| 04-15        | language (14)                                                                                                                                                                                                |
| 04-17        | spurious buff application and removal events removed                                                                                                                                                         |
| 04-19        | agent lookup too lenient, fixed (the entry speaks of siege log spam)                                                                                                                                         |
| 04-30        | removal of the last stability stack no longer logged (until 2018-08-08)                                                                                                                                      |
| 05-04        | gadgets flagged by the upper half of `prof`, lower half species or pseudo id; gadgets give no crit or glance; duplicate gadget rows of the release before maybe fixed                                        |
| 05-05        | `prof` corrupted since the gadget change, fixed; gadget pseudo ids generated differently                                                                                                                     |
| 05-14        | game build (15)                                                                                                                                                                                              |
| 05-18        | derived single removals logged as `is_buffremove` 3                                                                                                                                                          |
| 05-23        | role hints fixed                                                                                                                                                                                             |
| 06-13        | point of view pointed to the wrong agent, fixed                                                                                                                                                              |
| 06-20        | deferred state changes with a zero `dst_agent`, fixed                                                                                                                                                        |
| 07-02        | realtime API released; `ENTERCOMBAT` `dst_agent` is the subgroup                                                                                                                                             |
| 07-11        | log start and end carry the two timestamps                                                                                                                                                                   |
| 07-17        | workaround for characters whose log lacked data since the game build of 07-11                                                                                                                                |
| 07-31        | second cancel detection; some skill transitions may still lack a cast end                                                                                                                                    |
| 08-11        | `is_shields`                                                                                                                                                                                                 |
| 08-12        | glance set on a killing strike, fixed                                                                                                                                                                        |
| 08-15        | health update never 0                                                                                                                                                                                        |
| 08-19        | agent stats from `int32[3]` to `int16[6]`, concentration added                                                                                                                                               |
| 08-25        | manual logging removed                                                                                                                                                                                       |
| 09-05        | reward (17); `is_elite` is the elite spec id                                                                                                                                                                 |
| 09-07        | crit on gadget strikes                                                                                                                                                                                       |
| 09-13        | a dodge is followed by a reset                                                                                                                                                                               |
| 09-14        | guild and subgroups missing after a secondary log start, fixed                                                                                                                                               |
| 09-23        | account names broken since 09-22, fixed                                                                                                                                                                      |
| 09-30        | reset carries the skill id                                                                                                                                                                                   |
| 10-10        | removal `buff_dmg` is the longest stack for intensity buffs; reset after a dodge carries the dodge id                                                                                                        |
| 10-13        | a player target got the profession of the source, fixed                                                                                                                                                      |
| 11-22        | skill ids not set, fixed                                                                                                                                                                                     |
| 12-25        | flanking arc fixed                                                                                                                                                                                           |

`WEAPSWAP` (11) and `MAXHEALTHUPDATE` (12) appear this year without an
entry (open question 12), and so does `SHARDID` (16). Two lines of
2017-05-16 hint at them: weapon swap moves from polling to an engine
hook, and the shard id joins a tooltip.

### 2018, G1 then G2

| Date         | Change                                                                                                                                                                                                                                                                                                                                                                     |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 04-24        | `BUFFINITIAL` (18), duration 0                                                                                                                                                                                                                                                                                                                                             |
| 05-22        | result `0x10` on gadget strikes, fixed                                                                                                                                                                                                                                                                                                                                     |
| 06-26        | position (19) and velocity (20); `buff_dmg` 0 on resisted ticks; ticks of 0 damage logged (maybe)                                                                                                                                                                                                                                                                          |
| 06-28        | removals reporting 0, maybe fixed                                                                                                                                                                                                                                                                                                                                          |
| 07-10        | revision 1 struct behind the `new_cbtevent` ini key; offset 62 (byte 63 in the changelog) becomes `is_offcycle` for buff ticks; the README documents both structs                                                                                                                                                                                                          |
| 07-18        | facing (21)                                                                                                                                                                                                                                                                                                                                                                |
| 07-24        | result 9 downed; killing blows from every source; raid adds tracked for position; concentration no longer 0                                                                                                                                                                                                                                                                |
| 07-25        | barrier flag of condition damage fixed; negated condition damage sometimes not logged, fixed                                                                                                                                                                                                                                                                               |
| 08-08        | single removals logged for stability, last stack included again; gadget pseudo ids generated differently; gadgets get a position and facing when added                                                                                                                                                                                                                     |
| 10-02        | revision 1 is the default, with `dst_master_instid` and 32-bit `skillid` and `overstack_value`; strikes carry the barrier part and the downed flag; `BUFFINITIAL` gives the remaining duration; NPC stats capped; `iff` of characters from content affinity; log triggers for gadgets and for all players in combat; team change (22), attack target (23), targetable (24) |
| 10-03        | events against level 1 targets ignored                                                                                                                                                                                                                                                                                                                                     |
| 10-08        | map id (25)                                                                                                                                                                                                                                                                                                                                                                |
| 10-10        | overstack detection broken for an unknown span, fixed                                                                                                                                                                                                                                                                                                                      |
| 10-16        | logs without any player, fixed                                                                                                                                                                                                                                                                                                                                             |
| 10-17, 10-18 | unspecified log fixes after the optimization builds of the weeks before                                                                                                                                                                                                                                                                                                    |
| 11-25        | flanking from the line between the two agents; another attempt at agents showing as 0 when destination                                                                                                                                                                                                                                                                     |
| 12-11        | `REPLINFO` (26); `.evtc.zip` renamed `.zevtc`; damaging events logged outside combat; skill cancels not detected, fixed                                                                                                                                                                                                                                                    |
| 12-12        | log start not initialized, "fixed" (the quotes are the entry's)                                                                                                                                                                                                                                                                                                            |
| 12-14        | trackable ids in `pad61` to `pad64` on applications and single removals; single removals for every buff; `STACKACTIVE` (27); duration changes through the application event; `iff` unknown without agents                                                                                                                                                                  |

### 2019

| Date         | Change                                                                                                                                                 |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 01-08        | `is_shields` of an application is its active state, no `STACKACTIVE` for those; `STACKRESET` (28)                                                      |
| 01-09        | duration changes had source and destination swapped since 01-08, fixed                                                                                 |
| 01-10        | strict validity checks dropped (Keep Construct phases were lost)                                                                                       |
| 02-01        | most gadgets were missing from the agent table, fixed                                                                                                  |
| 03-05        | WvW logs, without relative stats or irrelevant agents                                                                                                  |
| 03-09        | instance logs ended by the WvW rules (Deimos), fixed                                                                                                   |
| 03-29        | guild (29); account and subgroup only for squad members; relative stats for the whole squad, WvW included                                              |
| 04-08, 04-23 | position, velocity and facing, then health and max health, reset at log start for all agents                                                           |
| 08-27        | initial position events could be missing, fixed                                                                                                        |
| 09-18        | `STACKACTIVE` `skillid` is the buff; gadgets as log targets                                                                                            |
| 10-17        | missing log end, fixed                                                                                                                                 |
| 11-07        | `is_activation` 2 unused; cast `buff_dmg` redefined on start and end                                                                                   |
| 12-25        | `BUFFINFO` (30), `BUFFFORMULA` (31), `SKILLINFO` (32), `SKILLTIMING` (33); the log target is the lowest visible species id; internal agent cap removed |

### 2020

| Date  | Change                                                                                                                                                                                                                                                   |
| ----- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 03-30 | targetable events of attack targets were missing, fixed                                                                                                                                                                                                  |
| 04-28 | `BUFFINFO` stacking type was not set, fixed, and `pad62` added; role hints fixed; duplicate casts without source filtered; overstack counted the stack being added, fixed; WvW logs built like the others, around the player instead of the squad centre |
| 05-06 | breakbar state (34) and percent (35); `BUFFFORMULA` extended; log end possibly fixed; unknown agents looked up in the game agent array                                                                                                                   |
| 05-13 | error (36)                                                                                                                                                                                                                                               |
| 06-09 | tag (37)                                                                                                                                                                                                                                                 |
| 06-23 | an agent rarely missing all its strikes, fixed                                                                                                                                                                                                           |
| 07-18 | tag events sometimes missing, fixed                                                                                                                                                                                                                      |
| 08-15 | minion data missing for some builds, fixed                                                                                                                                                                                                               |
| 11-17 | result 10, breakbar damage                                                                                                                                                                                                                               |
| 12-01 | barrier percent (38); in instances, events no longer have to happen near the squad                                                                                                                                                                       |

### 2021

| Date         | Change                                                                                                                         |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------ |
| 01-02        | `is_shields` on single removals; stability-like removals counted as intensity                                                  |
| 02-23        | removals of squad buffs count as squad events; `BUFFFORMULA` conditions                                                        |
| 03-29        | `STATRESET` (39)                                                                                                               |
| 04-29        | `EXTENSION` (40), `APIDELAYED` (41); banners get their owner; `is_shields` 2 on stability-like applications                    |
| 05-11        | `is_moving` bit 1 for the target (never set, see 2024-07-16); `cbtbuffcycle`; `BUFFINFO` `overstack_value` is the duration cap |
| 05-25        | condition ticks coalesced; announces team change for hostile targets, shipped with the release before (05-14)                  |
| 05-26        | announces result 11, shipped on 05-25                                                                                          |
| 05-30        | cancel times at 0 in the release of 05-29, fixed                                                                               |
| 08-28        | `pad61` downed flag on buff ticks                                                                                              |
| 09-22        | events no longer have to happen near the squad, in any mode; breakbar damage against gadgets                                   |
| 09-23        | cast, cancel and reset detection reworked; team fixed; out of range skill ids fixed                                            |
| 09-27, 09-28 | cast finish and cancel fixes                                                                                                   |
| 10-19        | compression built in and always on                                                                                             |
| 12-14        | breakbar damage uses the last damaging skill id; a skill id error of the release before patched                                |

### 2022

| Date  | Change                                                                                                                                                                                     |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 02-28 | `INSTANCESTART` (42); `time` taken at event creation; map logs                                                                                                                             |
| 03-04 | custom skill ids start at 23275 instead of 65001                                                                                                                                           |
| 03-08 | announces that the release of 03-06 deleted `ATTR_CUST_FLATINC`                                                                                                                            |
| 05-20 | `TICKRATE` (43), `LAST90BEFOREDOWN` (44)                                                                                                                                                   |
| 06-28 | cast start carries the target location; `EFFECT` (45); `IDTOGUID` (46); missing cast ends of second-use skills and weapon stow added; skill ids still read as 16 bits in some cases, fixed |
| 06-29 | `BUFFINITIAL` skills missing from the skill table, fixed                                                                                                                                   |
| 07-01 | `IDTOGUID` for markers; players missing from some arena logs, possibly fixed                                                                                                               |
| 07-09 | `IDTOGUID` defect fixed, earlier ones to be ignored                                                                                                                                        |
| 07-19 | rocket and sylvari turrets get their owner (engineer turrets had it from 2020-05-06)                                                                                                       |
| 08-23 | stow and draw names swapped back; tag `buff` flag; tags refreshed at squad combat start                                                                                                    |
| 09-14 | missing tag events, fixed                                                                                                                                                                  |
| 10-07 | map logging independent from boss logging                                                                                                                                                  |
| 11-11 | logs include the events before the boss fight (Xera); `LOGNPCUPDATE` (47)                                                                                                                  |
| 11-29 | boss named by a temporary name, maybe fixed; boss setting rules changed, again on 12-13                                                                                                    |
| 12-13 | `is_flanking` is an angle                                                                                                                                                                  |
| 12-23 | a generic log is promoted on a damaging strike by a squad member                                                                                                                           |

### 2023

| Date  | Change                                                                                                         |
| ----- | -------------------------------------------------------------------------------------------------------------- |
| 01-10 | `LOGNPCUPDATE` agent and gadget flag; boss setting rules changed again                                         |
| 01-11 | wrong boss set by the release before, maybe fixed                                                              |
| 02-02 | logs wrongly typed WvW, fixed                                                                                  |
| 03-01 | `EXTENSIONCOMBAT` (49); initial events missing after a boss stat reset, fixed                                  |
| 07-18 | players join the table only when in combat; `EFFECT` retired, `EFFECT2` (51); fractal scale (50)               |
| 07-19 | `EFFECT2` orientation re-encoded; language and effect GUIDs broken the day before, fixed                       |
| 07-26 | map log not reset in short instances, fixed                                                                    |
| 08-22 | `EFFECT2` platform flag                                                                                        |
| 09-02 | duration change events carried wrong values, fixed                                                             |
| 09-12 | every polled state (position, velocity, facing, health, max health) reset at each spawn, not only at log start |
| 11-07 | duration change `value` possibly fixed again; `BUFFINITIAL` original duration                                  |
| 11-10 | agents linger after despawn, late events keep their agent                                                      |
| 11-13 | subgroups un-broken; the entry does not say since when                                                         |
| 12-13 | `EFFECT2` duration no longer clamped to 16 bits                                                                |
| 12-16 | out of range skill ids possibly fixed again                                                                    |

### 2024

| Date  | Change                                                                                                                                                                                                                                                                                |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 02-05 | ruleset (52); a ruleset buff makes the log a WvW log                                                                                                                                                                                                                                  |
| 03-28 | squad ground markers (53); marker additions and removals                                                                                                                                                                                                                              |
| 03-30 | marker event spam of the release before, fixed                                                                                                                                                                                                                                        |
| 04-18 | markers rewritten as a 0 then one event each                                                                                                                                                                                                                                          |
| 04-21 | placeholder skills for skill id 0                                                                                                                                                                                                                                                     |
| 04-24 | team events of enemies were missing, fixed                                                                                                                                                                                                                                            |
| 06-04 | minions can be the log target                                                                                                                                                                                                                                                         |
| 06-09 | initial active flag of some buffs fixed                                                                                                                                                                                                                                               |
| 06-12 | the kinds the README marks "limited to agent table" written for every agent of the table in every mode; new agent table rules; `LAST90BEFOREDOWN` removed; team change at despawn with the old team; targetable on characters; `ENTERCOMBAT` profession and spec; run-on boss warning |
| 06-13 | dead/down swap of 06-12 and untagged team changes at the end of the log, fixed                                                                                                                                                                                                        |
| 06-14 | `ARCBUILD` (54); hitbox widths missing in the release before, fixed                                                                                                                                                                                                                   |
| 06-18 | squad members missing from the table and ground marker initial positions, possibly fixed                                                                                                                                                                                              |
| 06-19 | events with zero source and destination wrongly filtered, fixed                                                                                                                                                                                                                       |
| 06-27 | result 12 crowd control; glider (55); `WEAPSWAP` old set                                                                                                                                                                                                                              |
| 07-09 | crowd control `value`; `STUNBREAK` (56); `INSTANCESTART` socket                                                                                                                                                                                                                       |
| 07-16 | `is_moving` bit 1 set                                                                                                                                                                                                                                                                 |
| 08-20 | list of the skills whose definitions are always written, updated                                                                                                                                                                                                                      |
| 10-30 | effects on agents get a trackable id and an end; `IDTOGUID` extra fields                                                                                                                                                                                                              |
| 11-20 | stray `ENTERCOMBAT`, fixed                                                                                                                                                                                                                                                            |
| 12-10 | default effect durations unscaled                                                                                                                                                                                                                                                     |
| 12-11 | markers broken by the release before, fixed                                                                                                                                                                                                                                           |

### 2025

| Date         | Change                                                                                                          |
| ------------ | --------------------------------------------------------------------------------------------------------------- |
| 01-01        | periodic state changes no longer throttled by the stats cycle, sparser before                                   |
| 03-13        | instance id kept when the agent lookup fails; WvW logs end without a range check                                |
| 03-15, 03-17 | missing state changes fixed; squad combat start and end in map logs from 03-15                                  |
| 04-20        | agent table needs damage interaction                                                                            |
| 04-28        | that rule partly reverted; content types 2 and 3, whose ids differ between the Chinese client and the others    |
| 05-25        | missiles (57 to 59)                                                                                             |
| 06-03        | `EFFECT2` replaced by 60 to 63, with a scale; missile radius fixed; `EXITCOMBAT` subgroup and spec              |
| 07-08        | effects without agent were missing, fixed                                                                       |
| 07-29        | map logs only on listed maps, with position and the other sampled states outside combat too; `BUFFINFO` `pad63` |
| 08-06        | `SQCOMBATSTART` `dst_agent` announced; no data kept when logging is off at the trigger                          |
| 08-09        | logs outside instances were not saved, fixed; species id of the header not set by the release before, fixed     |
| 08-19        | `IIDCHANGE` (64)                                                                                                |
| 08-27        | `MAPCHANGE` (65); `IIDCHANGE` for the POV; GUID events in map logs                                              |
| 08-29        | `EARLYEXIT` reverted; `SQCOMBATEND` map exit bit; missing player spawns possibly fixed                          |
| 09-08        | map exit bit on map logs too                                                                                    |
| 09-13        | team and spec fixed again; `BUFFFORMULA` original attribute                                                     |
| 09-23        | no spawn, despawn or iid change outside the instance map                                                        |
| 10-09, 11-04 | more skills and buffs whose definitions are always written                                                      |
| 10-28        | guild GUID fixed                                                                                                |

### 2026, G2 then G3

| Date  | Change                                                                                                                                                                                                                                                                                    |
| ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 02-03 | defiance bar events of gadgets fixed; strikes with skill id 0, ignored until then, logged under 23301                                                                                                                                                                                     |
| 02-26 | one run-on warning per boss, `g` suffix for gadgets; `CONTENTLOCAL_TEAM`; `SHARDID` upper bits                                                                                                                                                                                            |
| 04-14 | `STATRESET` removed; gadget interaction and emotes as casts; cast end `is_activation` 6; `MAPID` map type                                                                                                                                                                                 |
| 04-22 | cast stop reasons adjusted                                                                                                                                                                                                                                                                |
| 05-07 | G3: casts and buffs as state changes (67 to 72), `cbtbuffcycle` merged into `cbtresult`, flags and location dropped from cast events; 73 to 75; float and sink split; emote, stow, draw, dodge and control effect detection fixed                                                         |
| 06-02 | 76 to 78; targetable rules; more events can cancel a cast; `WVWOBJECTIVESTATUS` upgrade progress; defiance damage to gadgets fixed again, without the skill id overrides                                                                                                                  |
| 07-01 | 79 to 84, `RATEHEALTH` retired; missile remove position and target; hitbox height dropped (0 in the logs since build 20260602); needless health and barrier events on gadgets without health fixed; siege detached from its owner                                                         |
| 07-02 | missing skill list, fixed: the release of 07-01 wrote a skill table of one row and no buff or skill definition                                                                                                                                                                            |
| 07-08 | max health of 1 on agents without health, fixed                                                                                                                                                                                                                                           |
| 08-11 | 85, 86; player address from a pseudo account value                                                                                                                                                                                                                                        |
| 09-15 | 87; ping in `TICK`; `JUMP` was not written until now; `WVWTEAMS` moved to the end of the log; garbage position events maybe fixed; gadget pseudo ids no longer read uninitialized data; in logs outside instances, removals and cast ends of outsiders logged when they concern the squad |

## Build thresholds

An index of the main rules above that need a comparison with
`Header.Build`, sorted by build; the History columns hold the rest. A new
kind is listed only when it changes how something else is read: its
presence in a log is enough otherwise. A release date stands for its
build and README builds are marked. What is marked "measured" comes from
the logs, not from the sources. On the logs measured the header is a
release date, now and then the day after, and for 5 builds out of 48 a
date without a changelog entry (see the introduction), so a header dated
the day after a row marked "this one build" can be that build or the fix
of the next day. The README builds show the other direction: a build
slightly older than a release can already behave the new way.

| Build           | From this build on                                                                                                                                                         |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 20170214        | agents can be tracked across a log; account name in the name block                                                                                                         |
| 20170216        | subgroup in the name block; `is_flanking`                                                                                                                                  |
| 20170217        | health and max health in `dst_agent`                                                                                                                                       |
| 20170404        | cast start `value` is the expected duration                                                                                                                                |
| 20170430        | no removal of the last stability stack, until 20180808                                                                                                                     |
| 20170504        | gadgets flagged by the upper half of `prof`; this one build corrupts `prof`                                                                                                |
| 20170518        | derived single removals are `is_buffremove` 3                                                                                                                              |
| 20170702        | `ENTERCOMBAT` `dst_agent` is the subgroup                                                                                                                                  |
| 20170711        | log start and end carry the timestamps                                                                                                                                     |
| 20170811        | `is_shields`                                                                                                                                                               |
| 20170819        | agent stats as `int16[6]`                                                                                                                                                  |
| 20170905        | `is_elite` is the elite spec id                                                                                                                                            |
| 20170930        | reset carries the skill id                                                                                                                                                 |
| 20171010        | removal `buff_dmg` is the longest stack for intensity buffs; the reset after a dodge carries the dodge id                                                                  |
| 20180710        | `is_offcycle` on buff ticks; revision 1 possible                                                                                                                           |
| 20180724        | result 9, result 8 from every source; concentration set                                                                                                                    |
| 20180808        | server single removals for stability, last stack included                                                                                                                  |
| 20181002        | revision 1 by default; barrier part and downed flag on strikes; `BUFFINITIAL` remaining duration; `iff` from content affinity                                              |
| 20181125        | flanking from the line between the two agents                                                                                                                              |
| 20181211        | damaging events outside combat                                                                                                                                             |
| 20181214        | trackable ids; server single removals for every buff; duration changes; `iff` unknown without agents                                                                       |
| 20190108        | `is_shields` active state on applications; this one build swaps the agents of duration changes                                                                             |
| 20190329        | account and subgroup only for squad members                                                                                                                                |
| 20190918        | `BUFFACTIVE` `skillid` is the buff                                                                                                                                         |
| 20191107        | cast `buff_dmg` meanings; `is_activation` 2 unused                                                                                                                         |
| 20201117        | result 10                                                                                                                                                                  |
| 20201201        | events away from the squad, in instances                                                                                                                                   |
| 20210102        | `is_shields` on single removals                                                                                                                                            |
| 20210429        | `is_shields` 2 on stability-like applications                                                                                                                              |
| 20210511        | `cbtbuffcycle` values in `is_offcycle`                                                                                                                                     |
| 20210525        | coalesced ticks; result 11                                                                                                                                                 |
| 20210828        | downed flag of ticks in `pad61`                                                                                                                                            |
| 20210922        | events away from the squad, in every mode                                                                                                                                  |
| 20220228        | `time` taken at event creation                                                                                                                                             |
| 20220304        | custom skills from 23275                                                                                                                                                   |
| 20220306        | `ATTR_CUST_FLATINC` gone from the formula attributes                                                                                                                       |
| 20220628        | cast start `dst_agent` is no agent: two floats by the README, a pointer-like value in the logs from 20230114 on                                                            |
| 20220709        | `IDTOGUID` can be trusted                                                                                                                                                  |
| 20220823        | markers written at squad combat start, with the commander flag                                                                                                             |
| 20221213        | `is_flanking` is an angle                                                                                                                                                  |
| 230716 (README) | kind 51 instead of 45                                                                                                                                                      |
| 20230719        | final orientation encoding of kind 51                                                                                                                                      |
| 20230902        | duration change values fixed, possibly again on 20231107                                                                                                                   |
| 20231107        | `BUFFINITIAL` original duration                                                                                                                                            |
| 20231110        | agents linger after despawn                                                                                                                                                |
| 20231213        | kind 51 duration on 32 bits                                                                                                                                                |
| 20240328        | marker additions and removals                                                                                                                                              |
| 20240418        | markers written as a 0 then one event each                                                                                                                                 |
| 240529 (README) | no kind 44                                                                                                                                                                 |
| 20240612        | the kinds marked "limited to agent table" for every agent of the table; old team in `TEAMCHANGE`; profession and spec in `ENTERCOMBAT`; this one build swaps dead and down |
| 20240613        | this one build leaves most hitbox widths at 0 (measured)                                                                                                                   |
| 20240627        | result 12; old set in `WEAPSWAP`                                                                                                                                           |
| 20240709        | crowd control `value` is a duration                                                                                                                                        |
| 20240716        | `is_moving` bit 1                                                                                                                                                          |
| 20241030        | trackable id and end for effects on agents                                                                                                                                 |
| 20250313        | instance id kept when the agent lookup fails                                                                                                                               |
| 250526 (README) | kinds 60 to 63 instead of 51                                                                                                                                               |
| 20250603        | subgroup and spec in `EXITCOMBAT`                                                                                                                                          |
| 20250708        | most `BUFFINITIAL` events are empty, until 20250913 (measured; no log between 20240723 and this build)                                                                     |
| 20250819        | the address of a player can change after its spawn (`IIDCHANGE`)                                                                                                           |
| 20250829        | `SQCOMBATEND` `dst_agent` bit 0                                                                                                                                            |
| 20251118        | hitbox height 0 for players and NPCs (measured)                                                                                                                            |
| 20260203        | skill id 0 strikes logged, under 23301                                                                                                                                     |
| 20260226        | content type 4 is a team, until 20260604 (measured)                                                                                                                        |
| 260402 (README) | no kind 39                                                                                                                                                                 |
| 20260414        | cast end `is_activation` 6; map type in `MAPID`; content type 5 is an emote, until 20260604 (measured)                                                                     |
| 260501 (README) | G3                                                                                                                                                                         |
| 20260507        | content type 6 is a transformation (measured)                                                                                                                              |
| 20260602        | hitbox height 0 for every agent (measured)                                                                                                                                 |
| 20260701        | no health or barrier update for an agent without health; until then nearly every barrier update is one and holds no percentage (measured)                                  |
| 20260701        | this one build writes a skill table of one row and no buff or skill definition (measured)                                                                                  |
| 20260702        | the custom skills of arcdps have no row in the skill table (measured)                                                                                                      |
| 20260915        | ping in `TICK`; half of the teleports without a target (measured)                                                                                                          |

## What the module assumes

- `parse.go` rejects any header revision other than 1.
- `timeline.Build` returns `ErrLegacyLog` below `MinBuild` (20240613).
  `TestBuildRejectsLegacyLogs` pins it, and `README.md`, `AGENTS.md`,
  `timeline/README.md`, `timeline/doc.go` and
  `timeline/example_readme_test.go` state the floor.
- For a log without kinds 67 to 72 and older than 20260501,
  `timeline/legacy.go` gives each event the kind G3 writes for the same
  fact, by the [typing rules](#event-typing-before-20260501); `scan` and
  `fill` switch on that kind, so both generations share one path.
- `startCast` takes `dst_agent` as the target agent. That field is no
  agent in the G2 logs the module reads, and a cast there has no target.
- `newHit` reads the downed flag from `is_offcycle` and keeps `result` as
  it is. On a G2 buff tick `legacyTick` reads the cycle of `is_offcycle`
  into `result` 14 to 18, a `result` of 1 to 4 into 6, and the downed
  flag from `pad61`.
- Stacks are matched by the trackable id in `pad61` to `pad64`;
  `BUFFACTIVE` reads it from `dst_agent`. G2 stores both in the same
  places from 2018-12-14. Before that date no event names a stack.
- Effects are built from kinds 60 to 63 and from kind 51. Kind 45 is not
  read.
- `BUFFINITIAL` events without `skillid` are skipped, a health or barrier
  update above 100% is no sample, `STATRESET` (39) names no agent, and a
  log older than 20240627 gets no weapon set before its first swap.
- The timeline reads kind 40 as the registration of an extension and
  hands only kind 49 to its decoder.
- Nodes expose their raw event (`Hit.Event`, `Cast.Start`, `Cast.Stop`,
  `BuffStack.Apply`, `BuffStack.Changes`, `BuffStack.Remove`) and
  `Events.Of` filters raw events by kind. On a G2 log the events of
  casts, buff applications, duration changes and removals are kind 0.
- When no `LOGNPCUPDATE` names a boss, `finish` takes a header species id
  above 2 as the boss. In logs older than the README of 2022-11, 1 is a
  generic or a manual log and 2 has no documented meaning.
- `HitboxHeight` keeps its name. The arcdps author says it never was a
  height, and the logs hold 0 there for players and NPCs from build
  20251118, for every agent from 20260602.
- `ContentKind` and `evtc.ContentLocal` follow the logs, not the README:
  team 4, emote 5, transformation 6.
- A `TELEPORT` without a target is no position sample: it breaks the next
  one.
- `ENTERCOMBAT` without profession is tolerated.
- The 10,565 logs measured from build 20240613 on build without an
  invariant failing, and each of the 5,682 G3 ones builds the same graph
  once written the G2 way. The 8 logs of build 20230114 are refused.

Nothing above needs an exported identifier to go or change type. Kind
values are stable, a revision 0 event widens into `evtc.Event` without
loss (the nine tracking bytes aside), and the G2 cast end values are the
G3 ones. What old logs needed on top was additive and is there for the
floor of 20240613: the `cbtbuffcycle` values (`evtc.BuffCycle`), the
payload of kind 51 and a meaning for `ActivationStartDefunc` and
`ActivationQuicknessDefunc`. The payload of kind 45 is not.

### Timeline data and the builds that carry it

"As G3" means older logs hold the data as G3 does, "none" that they do
not hold it.

| Data                                                                         | Kinds                | From                               | Older logs                                                                                                                                                                                   |
| ---------------------------------------------------------------------------- | -------------------- | ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| hits                                                                         | 0                    | G1                                 | barrier part and downed flag from 2018-10-02, downed on ticks from 2021-08-28, target moving from 2024-07-16, results 9 to 12 from their release                                             |
| casts                                                                        | 67, 68               | G1 through `is_activation`         | no skill on reset before 2017-09-30; control time from 2019-11-07; `dst_agent` is no target agent from 2022-06-28 (a pointer-like value in the logs from 20230114 on), and is unknown before |
| buff stacks                                                                  | 18, 27, 28, 69 to 72 | 2018-12-14 through the G2 typing   | applications and removals without trackable id before; single removals sent by the server missing before 2018-08-08, stability only until 2018-12-14                                         |
| buff and skill definitions                                                   | 30 to 33             | 2019-12-25                         | none                                                                                                                                                                                         |
| effects                                                                      | 60 to 63             | 2025-06-03                         | kind 51 from 2023-07-18, kind 45 from 2022-06-28, none before                                                                                                                                |
| GUIDs                                                                        | 46                   | 2022-07-09                         | kind 46 from 2022-06-28, to be ignored; none before                                                                                                                                          |
| missiles                                                                     | 57 to 59             | 2025-05-25                         | none                                                                                                                                                                                         |
| positions, velocity, facing                                                  | 19 to 21             | 2018-06-26, facing 2018-07-18      | players and boss only, then adds; every agent of the table from 2024-06-12                                                                                                                   |
| health                                                                       | 8                    | 2017-02-15                         | boss only in 2017; the changelog never says when the others joined                                                                                                                           |
| max health                                                                   | 12                   | 2017                               | as G3                                                                                                                                                                                        |
| barrier                                                                      | 38                   | 2020-12-01                         | none                                                                                                                                                                                         |
| defiance bar                                                                 | 34, 35               | 2020-05-06                         | as G3 from build 20230114 on, state in `value`; unknown before (open question 13)                                                                                                            |
| life states, combat                                                          | 1 to 7               | G1                                 | as G3                                                                                                                                                                                        |
| team                                                                         | 22                   | 2018-10-02                         | no old team before 2024-06-12                                                                                                                                                                |
| weapon sets                                                                  | 11                   | 2017                               | no old set before 2024-06-27                                                                                                                                                                 |
| profession and spec over time                                                | 1                    | 2024-06-12                         | agent table only. Kind 2 carries subgroup and spec from 2025-06-03, which the timeline does not read                                                                                         |
| targetable                                                                   | 24                   | 2018-10-02                         | nothing on characters and players before 2024-06-12                                                                                                                                          |
| markers                                                                      | 37                   | 2020-06-09                         | one event per commander when it enters combat, at squad combat start from 2022-08-23; additions and removals from 2024-03-28, re-encoded 2024-04-18                                          |
| ground markers                                                               | 53                   | 2024-03-28                         | none                                                                                                                                                                                         |
| guild                                                                        | 29                   | 2019-03-29                         | none                                                                                                                                                                                         |
| rewards                                                                      | 17                   | 2017-09-05                         | as G3                                                                                                                                                                                        |
| gliding, stun breaks                                                         | 55, 56               | 2024-06-27, 2024-07-09             | none                                                                                                                                                                                         |
| map changes, iid changes                                                     | 65, 64               | 2025-08-27, 2025-08-19             | none                                                                                                                                                                                         |
| instance start, fractal scale, ruleset                                       | 42, 50, 52           | 2022-02-28, 2023-07-18, 2024-02-05 | none                                                                                                                                                                                         |
| extensions                                                                   | 40, 49               | 2021-04-29, 2023-03-01             | an addon could only write kind 40 before 2023-03-01 (open question 11)                                                                                                                       |
| stealth, transformation, jumps, teleports, ping, gadget animations and names | 73 to 87             | 2026-05-07 to 2026-09-15           | none                                                                                                                                                                                         |

## Possible floors

What the table above allows without changing an exported type. The steps
are those of the [proposed split](#proposed-split). Only builds from
20230114 on can be checked against logs (see [Logs](#logs)). The floor is
at 20240613, the oldest build with more than a handful of them: it rests
on 4,883 G2 logs.

| Floor          | Needs                                                                                                                                                                                                                                                                                                        | What the logs it adds lack                                                                                                                                                                                                                                                                                                           |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 20240613, done | the G2 typing, kind 51 and the defects of a few builds (see [What the module assumes](#what-the-module-assumes))                                                                                                                                                                                             | the target of a cast, and what a capability dates later, among them the set left by a weapon swap before 20240627, stun breaks and crowd control before 20240709, ends of effects on agents before 20241030, missiles before 20250525                                                                                                |
| 20181214       | the G2 typing, the state change variants and effects 45 and 51 (steps C, F, E), and the hitbox part of step G if those fields start later (open question 3). No decoder change: revision 1 is the default from 2018-10-02, and a later log with revision 0, if one exists (open question 10), stays rejected | nothing the API models, the hitbox fields aside: stacks have a trackable id and every single removal sent by the server is logged. Data that starts later stays at its zero value                                                                                                                                                    |
| 20170214       | also revision 0 decoding, the agent table variants and a decision on stacks (steps B, G, D)                                                                                                                                                                                                                  | a trackable id before 2018-12-14; the single removals sent by the server, none before 2018-08-08 and stability only until 2018-12-14; the removal of the last stability stack from 2017-04-30 to 2018-08-08: `BuffStack` can only be simulated or left unmatched. No positions before 2018-06-26, no elite spec id before 2017-09-05 |
| below 20170214 | out of reach: no G0 layout is documented                                                                                                                                                                                                                                                                     |                                                                                                                                                                                                                                                                                                                                      |

## Developer impact

Accepting older logs can hurt the user of the module in two ways. Data
can be missing: the API answers with zero values and empty queries, and
G3 logs differ from each other (a log of build 20260507 has no stealth,
no teleport and no ping). Or a field can be there with another meaning.
The G2 typing (step C) then translates it, or leaves the field of the
node at its zero value. The raw event behind the node keeps the G2
meaning, since `Timeline.Log` is never modified.

### Fields that change meaning

For logs from 20181214 on. Logs of 20181002 to 20181213 are revision 1
without trackable ids (see [Possible floors](#possible-floors)), and
revision 0 logs add their garbage bytes and their 16-bit fields. The
rules that concern builds from 20240613 on are in `timeline/legacy.go`;
the others wait for a lower floor.

| Field or kind                              | In G2                                                                                                                                                                                 | Rule                                                                                                                          |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| tick `is_offcycle`                         | the cycle; only zero or non-zero before 2021-05-11                                                                                                                                    | 0, 1, 3, 4 and 5 read as `result` 14 to 18; 2 and any non-zero value before 2021-05-11 have no G3 value, 15 being the closest |
| tick `pad61`                               | downed flag from 2021-08-28                                                                                                                                                           | read as `is_offcycle`; unknown before 20210828                                                                                |
| tick `result`                              | 1 to 4 mark invulnerability; the logs hold 1 and 3                                                                                                                                    | read as 6 (`ABSORB`), which G3 writes on such a tick                                                                          |
| cast start `dst_agent`, `overstack_value`  | no agent from 2022-06-28: two floats and a z by the README, a pointer-like value in the logs; unknown before                                                                          | not read: no target agent before G3                                                                                           |
| cast start `is_activation` 2               | a start under quickness until 2019-11-07                                                                                                                                              | read as a start                                                                                                               |
| cast `buff_dmg`                            | undocumented before 2019-11-07                                                                                                                                                        | not read before 20191107                                                                                                      |
| cast end `value`, `buff_dmg`               | the G3 pair, measured                                                                                                                                                                 | read as they are                                                                                                              |
| `skillid` 65001                            | dodge until 2022-03-04                                                                                                                                                                | read as 23275                                                                                                                 |
| duration change `value`, `overstack_value` | wrong values until 2023-09-02; `value` possibly until 2023-11-07                                                                                                                      | not read before 20230902; `value` possibly not before 20231107                                                                |
| `IDTOGUID` (46)                            | defective before 2022-07-09                                                                                                                                                           | not read before 20220709                                                                                                      |
| kinds 45 and 51                            | the older effect layouts; the duration of kind 51 is clamped to 16 bits until 2023-12-13                                                                                              | read into `Effect` nodes; a duration of 65535 is unknown before 20231213                                                      |
| `MARKER` (37)                              | one event per commander when it enters combat; at squad combat start and with the commander flag in `buff` from 2022-08-23; on every change from 2024-03-28, re-encoded on 2024-04-18 | read by build                                                                                                                 |
| `TEAMCHANGE` (22), `WEAPSWAP` (11) `value` | no old team before 2024-06-12, no old set before 2024-06-27                                                                                                                           | not read before those builds: no first span, 0 being a weapon set                                                             |
| crowd control `value`                      | undocumented before 2024-07-09                                                                                                                                                        | read as it is; `CapabilityCrowdControl` tells a log older than 20240709                                                       |
| formula attributes                         | `ATTR_CUST_FLATINC` deleted on 2022-03-06                                                                                                                                             | open question 6                                                                                                               |
| `BUFFINITIAL` (18) without `skillid`       | four out of five from build 20250708 to 20250913                                                                                                                                      | skipped                                                                                                                       |
| duration change with `value` 0             | types as a tick; build 20230114 only                                                                                                                                                  | read as a change when its pads hold a stack id                                                                                |
| `IDTOGUID` content types 4 and 5           | a team from 20260226 and an emote from 20260414, not the emote and transformation of the README                                                                                       | read as `ContentTeam` and `ContentEmote`                                                                                      |

### Capabilities and warnings

Zero values hide one thing: "this agent had no barrier" and "this log
cannot say" look the same. `capability.go` of package `evtc` tells them
apart on the decoded log, and `warning.go` says what is wrong with it.
`timeline.Build` asks it once and the timeline answers the same four
questions: `Has`, `Capabilities`, `Missing` and `Warnings`.

`Log.Has` is true when the build is at least `Capability.FirstBuild`, or
when the events prove the capability: by a kind, or for some by a field
value (a strike of result 10, bit 1 of `is_moving`, a trackable id on an
initial buff, the old team of a team change). Some of those proofs rest
on a field staying at 0 until the capability, which holds on the logs
measured (open question 15 says what those logs cannot reach). Both tests
are needed: a build slightly older than a release can already write its
kinds, and a fight where nobody used stealth has no kind 76 whatever the
build. `CapabilityGUIDs` needs both at once, the build and a kind 46:
older GUIDs are defective, and map logs hold none before 2025-08-27. When
`Has` is false, an empty result proves nothing. `Log.Missing` lists what
`Has` refuses.

The first build is the first one that writes usable data, not the release
of the kind: `JUMP` (86) was released on 2026-08-11 and never written
before 2026-09-15. The logs measured follow the changelog release by
release: over the 49 builds no capability is proved by a log older than
its first build, and each capability that events can prove is proved from
that build on, `RULESET` (52) aside, which no log measured holds. Build
20260507 lacks stealth, gadget animations and names, missile effects,
capture points, ticks, teleports, jumps, ping and gadget models. Build
20260816 still lacks the last three, and the 48 logs of build 20260915
have them: jumps in 40, ping and gadget models in all. From that build a
`WVWTEAMS` event closes every log, so `CapabilityWvW` is proved by PvE
logs too. Outside instances arcdps limits some kinds to the squad or to
the agent table, which a capability does not tell. The table of
`capability.go` holds the list, with the first build and the proving
kinds of each, the field proofs being in `observe`. The tables of
[cbtstatechange](#cbtstatechange) and [cbtresult](#cbtresult) and the
[Build thresholds](#build-thresholds) give the dates behind it.

`Log.Warnings` tells a person what is known to be wrong with one log, and
each warning has a code a program can test. It does not repeat the
`INTEGRITY` (36) messages of arcdps, which the timeline keeps in
`Integrity`. Four sources:

- A build date that is not a date of eight digits from 2016 on:
  capabilities then rest on the events alone, and `timeline.Build`
  refuses the log.
- A release with a known defect, from its release to the fix: 20190108 to
  20190109 (agents of duration changes swapped), 20210529 to 20210530
  (cancel times at 0), 20220628 to 20220709 (defective GUIDs), 20230110
  to 20230111 (possibly the wrong boss in the header), 20230718 to
  20230719 (language, effect GUIDs, orientation of kind 51), 20240329 to
  20240330 (possibly marker spam), 20240612 to 20240613 (dead and down
  swapped), 20240613 to 20240614 (most hitbox widths at 0), 20241210 to
  20241211 (markers), 20250525 to 20250603 (motion radius of missile
  launches), 20250806 to 20250809 (species id of the header). The list is
  not exhaustive: an entry that names no data gets no warning. A defect
  tied to a kind is only reported when the log holds that kind. A header
  can be dated the day after its release, so one dated the day of a fix
  is either build and is not reported, and one dated the first day of a
  defect may belong to a sound release of the day before, hence the
  "possibly" of 20240329. 20240613 needs none: the 78 logs of that date
  all lack most of their widths. Two more defects belong to revision 0,
  which `Parse` rejects: 20170504 (`prof`) and 20170922 (account names).
- A header species id of 1 before 20221126, the date the listings give
  the README that first calls 1 a WvW log: a generic log before, WvW or
  not. No changelog entry gives a build for that change.
- A skill table that holds none of the skills the events name: the
  release of 2026-07-01, fixed the next day, writes a table of one row,
  and no buff or skill definition.

On the 10,573 logs measured, the hitbox warning fires on the 78 logs of
build 20240613, the skill one on the 37 of build 20260701, and nothing
else does. At the floor of 20240613 the G2 typing leaves one thing
unread, the target of a cast, which `CapabilityTypedEvents` tells. A
lower floor will add one warning per field left unread.

## Open questions

What the sources do not answer. The logs measured settled 1, 2 and 4 and
narrowed 5, 7, 9, 13 and 15; most of the rest needs logs older than
20230114.

1. Settled: cast end `value` and `buff_dmg` are the same pair in G2 and
   G3 (see [Cast end](#cast-end)).
2. Settled: G3 writes `result` 6 on a buff tick negated by
   invulnerability (see [Buff tick](#buff-tick)).
3. Which build starts writing the hitbox fields? The `example.cpp` still
   published on 2018-05-14 has pads, the writer captured in 2022 has the
   fields.
4. Settled: content type 4 is a team, and the README is one short from
   there on (see [Others](#others)).
5. Did kinds 60 to 66 keep one layout between their release in 2025 and
   the README of 2026-05 that first documents them? From build 20250708
   on they did; the five weeks before are unknown. For 57 to 59 the
   READMEs of 2025-05 and 2026-05 can be compared: launch gained the
   motion radius and the first launch flag.
6. What are the `BUFFFORMULA` layouts of 2019-12-25, 2020-05-06 and
   2021-02-23, and where did `ATTR_CUST_FLATINC` sit before 2022-03-06,
   that is, which attribute ids moved?
7. What is in `dst_agent` of a cast start before 2022-06-28, and is it
   already the pointer-like value of build 20230114 between the two
   dates?
8. Does the delay of 2021-04-29 move buff applications in the file, or
   only in the realtime API?
9. Do builds exist between 20260501 and 20260507, and which typing do
   they use? The logs measured jump from 20260416 to 20260507.
10. Did the opt-in revision 1 of 2018-07-10 already have the layout of
    2018-10-02, whose entry says the struct was shuffled? And can a later
    build still write revision 0, since the published writer keeps the
    switch?
11. The timeline hands only kind 49 to a decoder. What did the healing
    stats addon write before 2023-03-01, when only kind 40 existed?
12. When were kinds 11 and 12 released? The changelog holes of early 2017
    hide it, and possibly other changes.
13. Does any build put the defiance bar state in `dst_agent`, as the
    READMEs from 2024-07 say? None from 20230114 on.
14. How to tell a gadget from an NPC before 2017-05-04, and does
    `is_elite == 0xFFFFFFFF` already mark non players then?
15. Do the fields the capability proofs of `capability.go` read stay at 0
    before their build: `value` of kinds 1, 11 and 22, `buff_dmg` and the
    pads of kind 18, `dst_agent` of kind 10? They do on the logs
    measured, which hold only 8 logs older than 20240613, all of build
    20230114.

## Proposed split

Each step names the sections it builds on. A comes first: the others
cannot be checked without its logs, which exist from build 20230114 on.
B, E, F and G do not depend on C, D builds on C, and H closes.

|     | Step                         | Sections                                                                                                                                                        | Outcome                                                                                                                                                                                                                                                                                                                                                                                           |
| --- | ---------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A   | Old logs and measurements    | [Generations](#generations), [Open questions](#open-questions)                                                                                                  | done from build 20230114 on, the two holes of [Logs](#logs) aside. Missing: one log per period before that (2017, 2018 before and after 2018-10-02, one per year from 2019 to 2022), and the open questions they would answer                                                                                                                                                                     |
| B   | Revision 0 decoding          | [File layout](#file-layout)                                                                                                                                     | `parse.go` accepts `header[12] == 0` and widens the event; fixture builder, unit tests, fuzz seed                                                                                                                                                                                                                                                                                                 |
| C   | G2 typing                    | [Event typing](#event-typing-before-20260501), [cbtresult](#cbtresult), [cbtactivation](#cbtactivation-now-cbtanimation), [cbtbuffcycle](#cbtbuffcycle-g2-only) | done from build 20240613 on, in `timeline/legacy.go`: casts, buff applications, changes, removals, ticks and strikes reach the same nodes as G3, with the raw event kept on each node, and the target of a cast, left unread, is told by `CapabilityTypedEvents`. Left: G1 and the G2 builds below the floor, with a `Warning` for each field left unread ([Developer impact](#developer-impact)) |
| D   | Stacks without trackable ids | [Buff removal](#buff-removal), [Buff application](#buff-application-and-duration-change)                                                                        | a decision and an implementation for logs before 2018-12-14: simulate stacks or expose applications and removals unmatched                                                                                                                                                                                                                                                                        |
| E   | Effects 45 and 51            | [Effects](#effects)                                                                                                                                             | kind 51 done. Left: `Effect` nodes from kind 45, GUIDs ignored in logs older than 2022-07-09                                                                                                                                                                                                                                                                                                      |
| F   | State change variants        | [cbtstatechange](#cbtstatechange), [Build thresholds](#build-thresholds)                                                                                        | payload rules by build for kinds 1, 2, 8 to 10, 18, 22, 24, 25, 34, 37, 46, and the downed and moving flags                                                                                                                                                                                                                                                                                       |
| G   | Agent table variants         | [Agent table](#agent-table-96-bytes-per-agent)                                                                                                                  | `is_elite` 0 or 1, gadgets before 2017-05-04, stats layout, hitbox, no `IIDCHANGE`                                                                                                                                                                                                                                                                                                                |
| H   | Floor and documentation      | [What the module assumes](#what-the-module-assumes), [Possible floors](#possible-floors)                                                                        | done for the floor of 20240613: `MinBuild` is the oldest build read and `ErrLegacyLog` the refusal below it, the README files and `AGENTS.md` say so, and the healing stats decoder passes its invariants on the 255 G2 logs that hold its events                                                                                                                                                 |
