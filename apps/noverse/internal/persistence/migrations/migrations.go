// Package migrations embeds the goose SQL migrations so the binary carries its
// own schema — no runtime filesystem dependency.
//
// # Implemented manifest (release order)
//
// File numbering is five-digit release order, NNNNN_snake_case.sql, with goose
// Up/Down markers. The implemented order diverges from the original plan
// allocation in reports/schema-and-contracts.md §3.11 (historical proposal,
// not the applied history):
//
//	00001 foundation (extensions + noverse_meta + schema.format)          ← plan 02
//	00002 accounts, credentials, sessions, devices, members, audit, blobs ← plan 03+04
//	00003 narrative domain, documents, streams, leases, guards            ← plan 05
//	00004 story documents (project_documents/story_artifacts split)       ← plan 05
//	00005 import ledger + library_settings                               ← plan 08
//	00006 story lease key                                                ← plan 06
//	00007 workers, jobs, audio_*, export_cache                           ← plan 00004
//	00008 mutable-row version/updated_at guards for 00002/00005 tables   ← audit S1
//	00009 audio_scripts status/source_sha256 CHECKs                      ← audit S8
//	00010 history mutability matrix + privilege-gated escape hatch       ← audit S13
//	00011 staged-upload job ownership + maintenance-only blob/cache DELETE ← GC fix
//	00012 shared identity per-app grants                                 ← verse SSO
//	00013 shared identity first-setup insert guard                        ← verse SSO
//	00014 hosted co-create drafts                                        ← old 00012
//	00015 deleting blob claims and publication fence                      ← old 00013
//	00016 admin storage settings and online store migration               ← old 00014
//	00017 resumable source purge progress                                 ← old 00015
//	00018 expired storage probe configuration cleanup                     ← old 00016
//	00019 shared session revocation reason/app                           ← verse SSO
//	00020 known-device tokens for the login limiter                      ← verse SSO
//
// Applied migration numbers are never renamed or edited. New work appends the
// next free number; postgres.RequiredMigrations derives the required set from
// this embed FS and readiness rejects a database missing any of them (newer
// additive schemas remain acceptable: expand-only).
//
// # Mutability matrix (audit S13)
//
//	Append-only history — UPDATE/DELETE only under the maintenance principal
//	  (noverse_migrate or a superuser) plus the maintenance marker:
//	    audit_logs, chapter_versions, import_items
//	Document-resettable engine streams — the document backend owns tail
//	  repair/reset; the transaction marker remains sufficient:
//	    story_events, checkpoints, agent_sessions, job_events
//	Business rows — version+1 and updated_at in every UPDATE (trigger backstop;
//	  repositories still CAS on the expected version):
//	    all other tables with a version column
//	Exempt bookkeeping — noverse_meta (no version column) and
//	  auth_invariant_lock (lock-only singleton).
//
// Migrations are expand-only while the file backend remains authoritative;
// Down sections are throwaway-only and must never destroy production data. The
// deployment playbook forbids `goose down` on any database with writers, and
// Down is not a production rollback mechanism.
package migrations

import "embed"

// FS contains the *.sql goose migrations, embedded at build time.
//
//go:embed *.sql
var FS embed.FS
