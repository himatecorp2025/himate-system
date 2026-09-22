# START-23.5 Acceptance — Dynamic Bilingual Business Model

## Purpose

START-23.5 closes the remaining dynamic-data localization gap. Static interface translation is not sufficient: business records created by administrators must retain independent English and Hungarian labels/descriptions and must resolve the correct legacy display fields for the active locale.

The authoritative stable identifiers remain language-neutral:
- partner category ID / slug;
- module-group key;
- module key;
- impact metric key;
- custom role key.

Localization never changes authorization, pricing, entitlement, identity or reporting keys.

## Locale contract

Backend locale resolution order:

1. `locale=hu_HU|en_US` query parameter;
2. `X-Himate-Locale` request header;
3. first `Accept-Language` value;
4. default `en_US`.

The shared implementation is `services/internal/common/locale.go`.

Every bilingual dynamic response retains both explicit language variants while the backward-compatible display field is resolved to the request locale:

```json
{
  "label": "Kulturális programok",
  "label_en": "Cultural Programs",
  "label_hu": "Kulturális programok"
}
```

If one language is missing only for legacy/backfilled data, display resolution falls back to the other language. New START-23.5 editor mutations require both language variants.

## Dynamic records in scope

### Partner categories

Persistence:
- `partners.categories.name_en`
- `partners.categories.name_hu`

API:
- `GET/POST /api/v1/partner-categories`

New category UI must collect both names.

### Module groups

Persistence:
- `catalog.module_groups.label_en`
- `catalog.module_groups.label_hu`

API:
- `GET/POST /api/v1/module-groups`
- `PATCH /api/v1/module-groups/{groupKey}`

New group UI must collect both labels.

### Modules

Persistence:
- `catalog.modules.label_en`
- `catalog.modules.label_hu`
- `catalog.modules.description_en`
- `catalog.modules.description_hu`

API:
- `GET/POST /api/v1/modules`
- `PATCH /api/v1/modules/{moduleKey}`
- partner-module commercial read models

Module create/edit UI must retain both language variants.

### Impact metric definitions

Persistence:
- `impact.metric_definitions.label_en`
- `impact.metric_definitions.label_hu`
- `impact.metric_definitions.description_en`
- `impact.metric_definitions.description_hu`

API:
- `GET/POST /api/v1/impact/definitions`
- impact summary display model

### Custom roles

Persistence:
- `identity.custom_roles.label_en`
- `identity.custom_roles.label_hu`
- `identity.custom_roles.description_en`
- `identity.custom_roles.description_hu`

API:
- `GET/POST /api/v1/admin/roles`
- `PATCH /api/v1/admin/roles/{roleKey}`

RBAC continues to resolve permissions exclusively by stable `role_key`. Localized role metadata cannot change effective permissions.

## Flutter contract

All normal API calls send `X-Himate-Locale: HimateI18n.activeLocale`.

Changing locale clears API cache so a cached English dynamic record cannot remain visible after switching to Hungarian, and vice versa.

The following editors must capture both languages:
- Add partner category;
- Add module group;
- Create/edit module;
- Create impact metric definition;
- Create/edit custom role.

## Required mutation proof

`scripts/smoke_start_23_5.sh` must prove on a real Compose database:

1. create a category with different EN/HU names;
2. read the same category under `en_US` and `hu_HU` and verify the resolved `name` switches;
3. create a module group with different EN/HU labels and prove locale readback;
4. create a module with distinct EN/HU label/description and prove registry readback;
5. prove the partner-module commercial matrix resolves that module/group label to Hungarian;
6. create an impact metric definition with distinct EN/HU metadata and prove definitions readback;
7. record an impact value and prove impact summary resolves the localized metric label;
8. create a custom role with distinct EN/HU metadata and prove role-list locale readback without changing permission keys;
9. PATCH the custom role bilingual metadata and prove both variants persist;
10. legacy single-label historical mutations continue to pass the complete START-01–23.4 regression suite.

## Definition of Done

START-23.5 is complete only when:
- Go tidy/vet/unit/race/build passes;
- shared locale helper unit tests pass;
- Flutter analyze/browser tests/release build passes;
- START-23/23.1/23.2/23.3/23.4 audits remain green;
- START-23.5 static bilingual-model audit passes;
- full START-01–23.4 Compose regression remains green;
- START-23.5 bilingual mutation smoke passes;
- functional matrix marks START-23.5 contracts mutation-proven;
- feature branch is merged into `develop` with no unique commits outside the merge path.

Production locale verification remains part of the final START-23.12 production-verification gate. START-24 remains blocked until START-23.6–23.12 close the remaining matrix blockers.
