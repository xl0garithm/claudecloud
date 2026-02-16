# ClaudeCloud Project Memory

## Meta-Plan: Phased Architecture Implementation
The project follows a phased architecture plan defined in `.FULL_ARCHITECTURE_PLAN.md`.

**Workflow for each phase:**
1. Create a detailed plan with sequential/parallel task breakdown
2. Execute tasks (code, tests, verification)
3. **Git commit after each meaningful stage** (batch completion, not every file) with detailed messages describing what changed and why
4. Check off completed items in `.FULL_ARCHITECTURE_PLAN.md`
5. Write explanations of implemented concepts to `docs/CONCEPTS.md`
6. Move to next phase with a fresh plan

## Git Commit Strategy
- Commit after each completed batch within a phase (e.g., "all contracts compile", "tests pass", "frontend wired up")
- Detailed commit messages: describe WHAT changed, WHY, and any notable design decisions
- This provides easy rollback points if something breaks in a later batch
- Never amend previous commits — always create new ones

**Phase Status:**
- Phase 1: Foundations & Infrastructure — COMPLETE
- Phase 2: Core Provisioning & Zero-Trust — COMPLETE
- Phase 3: MVP Dashboard & Billing — COMPLETE
- Phase 4: UI Layer & Multi-Platform — COMPLETE
- Phase 5: Reliability, Security & Launch — COMPLETE
- Phase 6: Scale & Monetization 2.0 — NOT STARTED

## Key Decisions
- **Provider abstraction**: `Provisioner` interface with feature toggle (`PROVIDER=docker|hetzner`). Local Docker is the default for dev/MVP (zero cloud spend). Hetzner is the production backend.
- Hetzner Cloud for production hosting (cost-effective, EU-based)
- Netbird Cloud for zero-trust networking in production (WireGuard-based mesh, skipped in Docker mode)
- Go for backend/control plane (Terraform integration via terraform-exec)
- Stripe for billing ($19–29/mo base + usage-based)
- Zellij + mosh for persistent CLI sessions
- Next.js for user portal/dashboard

## Project Structure
- `.FULL_ARCHITECTURE_PLAN.md` — Master architecture plan with checkboxes
- `docs/CONCEPTS.md` — Educational documentation of implemented concepts
