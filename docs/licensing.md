# Licensing

## Project license

The project is licensed under the GNU General Public License, version 3 or any later version (`GPL-3.0-or-later`). The complete license notice is in the repository root at [LICENSE](../LICENSE).

This choice reflects the repository's use of implementation and API patterns derived from Halo's GPLv3 repository. The project does not claim to be an official Halo distribution.

## Halo component attribution

The Console component package contains independently maintained Vue implementations of UI patterns whose reference implementations were audited against Halo 2.26-SNAPSHOT (`a559d155ab88e4883c78b823a6fc12e94213bf83`). The relevant upstream package declares MIT in `ui/packages/components/package.json`; Halo's repository-level distribution license is GPLv3. The affected files are listed in [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md).

The local implementations have been substantially adapted for this project: they use the project's dark-only Apple design tokens, Vue/TypeScript surface, and Console transport behavior. Future changes must not copy upstream source wholesale. Preserve the attribution and license notice when redistributing these components.

### File-level audit

The following audit was performed against the referenced revision. “Reference” identifies the closest upstream component surface; “adapted” means the local implementation was retained with project-specific structure and styling rather than vendoring the upstream file.

| Local file | Reference | Conclusion |
| --- | --- | --- |
| `VButton.vue` | `components/button/Button.vue` | adapted |
| `VCard.vue` | `components/card/Card.vue` | adapted |
| `VEmpty.vue` | `components/empty/Empty.vue` | adapted |
| `VEntity.vue` | `components/entity/Entity.vue` | adapted |
| `VEntityContainer.vue` | `components/entity/EntityContainer.vue` | adapted |
| `VEntityField.vue` | `components/entity/EntityField.vue` | adapted |
| `VLoading.vue` | `components/loading/Loading.vue` | adapted |
| `VModal.vue` | `components/modal/Modal.vue` | adapted |
| `VPagination.vue` | `components/pagination/Pagination.vue` | adapted |
| `VStatusDot.vue` | `components/status/StatusDot.vue` | adapted |
| `VSwitch.vue` | `components/switch/Switch.vue` | adapted |
| `VTabbar.vue` | `components/tabs/Tabbar.vue` | adapted |
| `VTag.vue` | `components/tag/Tag.vue` | adapted |
| `dialog.ts` | `components/dialog/dialog-manager.ts` | adapted |
| `toast.ts` | `components/toast/toast-manager.ts` | adapted |
| `FilterTag.vue`, `FilterDropdown.vue` | Halo tag/dropdown interaction patterns | local adaptation; no same-named upstream file |

The two local-only filter surfaces are recorded because their comments reference Halo interaction patterns, but the audit found no same-named file in the referenced package. `VFloatingDock.vue`, `VPageHeader.vue`, and `VSkeleton.vue` have no Halo reference marker and remain project-local.

## Stellar attribution

The extracted Stellar theme resources are from `hexo-theme-stellar` by xaoxuu and are MIT-licensed. Their notice is recorded in [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md).

## React Bits attribution

The Stellar 1.44 Galaxy and Card Hover effects include independent adaptations
of React Bits references by David HDev. They remain under the MIT terms and are
listed in [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md); the source URLs
are also retained in the JavaScript file headers.

## Dependency notices

Third-party packages that remain under their own licenses are governed by their package metadata and lockfiles. This document records the upstream code that is materially relevant to the repository's component audit; it is not a replacement for the license files distributed by those dependencies.
