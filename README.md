# Photo ID Quiz Builder

A desktop companion to the [Photo ID quiz runner](https://github.com/gbabineau/photo-id-wails). The runner plays quizzes; the builder creates the quiz JSON files it opens. Built with Go, Wails v2, and a React/TypeScript frontend.

## First slice: trip JSON to daily quizzes

Port of `photo_id/process_trip.py` from the legacy Python/Tkinter project. Given a trip file, it downloads (and caches) each hotspot's species frequencies and writes one runner-compatible quiz per itinerary day:

1. Average each species' frequency across the day's hotspots (only rows at or above `minimum_frequency`).
2. Add species listed in the day's `mentioned` array, marked `"Mentioned in trip data"`.
3. Attach eBird taxonomy (`speciesCode`, `sciName`, `taxonOrder`, family fields, and so on).
4. Drop species under 1%, sort taxonomically.
5. Keep each species only on the day where it is best placed: a day that mentions it wins, otherwise the higher frequency, otherwise the earlier day.

Output files are named `Day N with species 1-M.json`. A day with more than 40 species is split into the fewest balanced parts of at most 30 (`Day N with species 1-23.json`, `Day N with species 24-45.json`). A trip that produces more than one quiz also gets `All days with species 1-N.json`, one unsplit quiz with every species from every day in taxonomic order, for reviewing the whole trip. Each trip's quizzes go in their own folder, named after the trip's `name` (the file name is used if it has none), inside the shared quizzes folder (see "Where files live"). Quiz JSON uses `start_month`, `end_month`, `location`, `notes`, and `species` entries with `comName` plus optional `frequency`, `notes`, `speciesCode`, `sciName`, and taxonomy fields. Legacy uppercase `Notes` is read, and the first entry wins for duplicate common names.

### Inputs

- **Trip file**: JSON with `name`, `location`, `start_month`, `end_month`, `minimum_frequency`, and an `itinerary` of days (`day`, `Full day title` or `AM_title`/`PM_title`, `hotspots` with `hotspotId`, `mentioned`). Hotspot IDs may be `L485324` or an eBird hotspot URL.
- **Hotspot data**: downloaded from each hotspot's public eBird bar chart (no eBird login) and cached per hotspot under the user cache folder (`photo-id-quiz-builder/hotspots`) for 180 days. Weekly frequencies are combined over the trip's month range (wrapping past December), weighted by checklist counts. Spuhs, slashes, hybrids and forms are skipped. A hotspot that cannot be fetched produces a warning rather than a failure, and stale cache is used if a refresh fails.
- **Taxonomy**: see below.

eBird protects its pages with Anubis proof-of-work, so downloads run through headless Chromium (Playwright), as in the runner. The first download installs Chromium. Frequencies come from the bar chart, so unlike the legacy "targets" page they do not depend on your life list.

## Where files live

- **Trip files** are source, kept in their own repository outside this app. The file picker starts in the folder of the last trip you opened.
- **Quizzes** are written to `Documents\Photo ID\Quizzes\<trip name>\`, a folder both the builder and the runner can use. Change it with the Choose button, or set `PHOTO_ID_QUIZZES_DIR` (which takes precedence). Rebuilding a trip replaces its previous quizzes, including parts that are no longer needed, after you confirm in a dialog; the builder tracks what it wrote in a hidden `.quiz-builder.json` in the trip folder and never deletes other files.
- **Settings** are saved in `settings.json` under the user config folder (`photo-id-quiz-builder`). **Caches** (taxonomy, hotspot data) live under the user cache folder and can be deleted freely.

## Credentials and configuration

No credentials are stored in the repository or entered in the UI. The taxonomy is resolved in this order:

1. `EBIRD_TAXONOMY_PATH`: path to a taxonomy JSON file.
2. The user cache file `photo-id-quiz-builder/taxonomy.json` (under the OS user cache directory). eBird publishes a new taxonomy in late October, so a cached copy is reused until Oct 25; a copy downloaded during Oct 25-31 is replaced once from Nov 1.
3. Download from the eBird API using the `EBIRDAPIKEY` environment variable, then cache it. Rate-limit responses (429) are retried after the time eBird asks for.

Set the variable in your shell before launching, for example `$env:EBIRDAPIKEY = "..."` in PowerShell. Request a key from your eBird account. Set `PHOTO_ID_DEBUG=1` to log retries and delays.

### GitHub Actions secrets

The current workflow (`.github/workflows/quality.yml`) needs none: unit tests use fakes and a local HTTP server, and nothing launches a browser. If a live eBird test is added later, store the key as the repository secret `EBIRDAPIKEY` and expose it only to that step.

## Development

Requires Go, Node.js, and the Wails CLI.

```powershell
wails dev          # run with hot reload
wails build        # produce build/bin/photo-id-quiz-builder.exe
go test ./...      # backend tests
cd frontend; npm run build   # type-check and bundle the frontend
```

`frontend/wailsjs` is generated by Wails (`wails generate module`) and committed so the frontend builds without the CLI. Rerun it after changing methods or types bound in `app.go`.

## Legacy helper inventory and roadmap

| Legacy helper | Status |
| --- | --- |
| `process_trip.py` trip JSON to daily quizzes | Done (this slice), excluding live eBird scraping |
| `process_quiz.py` sort a quiz by taxonomy | Next: small, pure, reuses taxonomy loader |
| `process_quiz.py` split a quiz into parts | Next, alongside sorting |
| `process_quiz.py` eBird target list text to quiz | Later |
| `get_have_list.py` life list CSV | Later; the runner already reads life lists |
| `get_taxonomy.py`, `get_ebird_api_key.py` | Ported as `internal/taxonomy` |
| `get_size_data.py` Avonet size data | Not planned |
| `match_window.py`, `photo_id.py` (Tkinter UI, media) | Belongs to the runner |
