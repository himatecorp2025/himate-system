# START-23.11.3e Acceptance — New Partner Master-Data Onboarding

## Root-cause closure

The deployed **New Partner** action could appear to do nothing because the UI performed a blocking Catalog request for `/api/v1/modules` before calling `showDialog`. Any Catalog 502/error aborted the action before the modal existed.

START-23.11.3e removes all remote preconditions from opening the partner-creation dialog. The modal is a core Partners function and must open even when Catalog, Billing, CMS or other supplementary services are degraded.

## Onboarding model

Creating a partner now creates the authoritative company/master-data record first. Production provisioning, license evidence, invoices, module activation and environment launch remain separate controlled workflows in the partner workspace.

The New Partner modal captures:

### Company and legal identity
- display name;
- legal company name;
- brand / DBA name;
- partner category;
- company / registration number;
- tax / VAT ID;
- website;
- company phone;
- primary domain.

### Registered office and operational contacts
- country;
- state / region;
- city;
- postal code;
- address line 1 and 2;
- finance / billing contact and email;
- technical contact and email;
- marketing contact and email.

### Partner Portal and brand identity
- first Partner Portal Owner name;
- first Partner Portal Owner email;
- initial password under the existing password policy;
- optional PNG/JPEG/WebP company logo.

### Commercial defaults
- billing currency;
- activation fee;
- base monthly fee;
- minimum monthly commitment;
- quote / offer reference;
- internal notes.

## Failure isolation

Partner creation is not blocked by Module Catalog availability.

Category metadata is also non-blocking: if it is still unavailable when the operator opens the dialog, the UI uses the built-in `Other / cat_006` fallback and the category can be changed later.

After the core partner record is created, supplementary setup is attempted independently:
- first Partner Portal Owner;
- partner logo;
- commercial defaults.

A supplementary failure is reported as a warning and does not erase or hide the successfully created partner record.

## Partner logo contract

`POST /api/v1/partners/{partnerId}/logo` accepts multipart PNG/JPEG/WebP.

The gateway:
1. verifies the partner exists;
2. streams the file to the partner-scoped CMS media library;
3. registers the media as the explicit `logo` brand slot;
4. exposes that specific asset through the public CMS media route;
5. updates the authoritative partner `logo_url`.

Unassigned partner design media remains private.

## Persistence contract

`POST /api/v1/partners` now persists the same core company fields already supported by partner editing, including legal identity, tax/registration data, registered office and operational contacts.

This prevents the onboarding path from creating an incomplete record that later has to be manually reconstructed for billing or Partner Portal personalization.

## Automated evidence

- Deterministic frontend contract audit: `python3 scripts/audit_start_23_11_3e.py`
  - verifies the **New Partner** button is wired to `addPartner`;
  - verifies no awaited remote dependency can block `showDialog`;
  - verifies opening the modal contains no Module Catalog request;
  - verifies the complete master-data, Portal Owner and logo fields remain in the onboarding path.
- Existing Flutter browser suite continues to validate shared dialog/responsive behavior.
- Compose acceptance: `sh scripts/smoke_start_23_11_3e.sh http://127.0.0.1:8080`
  - creates a synthetic partner with complete company/legal/address/contact data;
  - creates its Partner Portal Owner;
  - uploads and publicly reads its partner-scoped logo;
  - verifies the logo URL is persisted;
  - signs in as that new partner and reads the same company identity from Partner Portal.

Release contract: `0.8.22-start-23.11.3e`.
