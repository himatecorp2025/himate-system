# START-23.11.3f Acceptance — Partner Category Resilience

## Defect

The deployed New Partner modal could show only **Other** and the message **Category service is still loading** even though the partners service owns six canonical categories.

The frontend created an Other-only snapshot whenever category metadata was not already available when the modal opened.

## Required behavior

The New Partner flow must always expose the six canonical system categories immediately:

- Classical Music / Klasszikus zene
- Fine Art / Képzőművészet
- Gallery / Galéria
- Theatre / Színház
- Cultural Organization / Kulturális szervezet
- Other / Egyéb

The live `/api/v1/partner-categories` registry remains authoritative for custom/dynamic categories. Remote rows are merged over the built-in system catalog.

The modal opens without waiting for the category service. While it is open, HIMATE refreshes the live category registry once. If that refresh fails, all six built-in categories remain usable instead of collapsing to Other-only.

The legacy **Category service is still loading** message is removed. If the live registry is unavailable, the UI states that built-in categories remain available.

## Automated evidence

- static audit: `python3 scripts/audit_start_23_11_3f.py`;
- Compose smoke: `sh scripts/smoke_start_23_11_3f.sh http://127.0.0.1:8080`;
- smoke asserts all six canonical categories are returned by the API;
- smoke creates a normal partner using `cat_003 / Gallery`, proving creation is not tied to `cat_006 / Other`.

Release contract: `0.8.23-start-23.11.3f`.
