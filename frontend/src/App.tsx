import {useState} from 'react'
import grackleAvatar from './assets/images/grackle-avatar.jpg'
import {
    ChooseFolder,
    ChooseTripFile,
    DefaultOutputDir,
    PreviewTrip,
    WriteTripQuizzes,
} from '../wailsjs/go/main/App'
import {main} from '../wailsjs/go/models'

function fileName(path: string): string {
    return path.split(/[\\/]/).pop() ?? path
}

function errorText(err: unknown): string {
    return err instanceof Error ? err.message : String(err)
}

export default function App() {
    const [tripPath, setTripPath] = useState('')
    const [hotspotDir, setHotspotDir] = useState('')
    const [outputDir, setOutputDir] = useState('')
    const [preview, setPreview] = useState<main.TripPreview | null>(null)
    const [written, setWritten] = useState<string[] | null>(null)
    const [error, setError] = useState('')
    const [busy, setBusy] = useState(false)
    const [open, setOpen] = useState<number | null>(null)

    const request = (output = outputDir) =>
        main.TripRequest.createFrom({tripPath, hotspotDir, outputDir: output})

    function reset() {
        setPreview(null)
        setWritten(null)
        setError('')
        setOpen(null)
    }

    async function pickTrip() {
        const path = await ChooseTripFile()
        if (!path) return
        setTripPath(path)
        setOutputDir(await DefaultOutputDir(path))
        reset()
    }

    async function pickFolder(title: string, set: (path: string) => void) {
        const path = await ChooseFolder(title)
        if (!path) return
        set(path)
        reset()
    }

    async function run<T>(action: () => Promise<T>): Promise<T | undefined> {
        setBusy(true)
        setError('')
        try {
            return await action()
        } catch (err) {
            setError(errorText(err))
        } finally {
            setBusy(false)
        }
    }

    async function onPreview() {
        setWritten(null)
        const result = await run(() => PreviewTrip(request()))
        if (result) {
            setPreview(result)
            setOpen(null)
        }
    }

    async function onWrite() {
        const paths = await run(() => WriteTripQuizzes(request()))
        if (paths) setWritten(paths)
    }

    const ready = tripPath !== '' && hotspotDir !== ''
    const totalSpecies = preview?.quizzes.reduce((sum, q) => sum + q.quiz.species.length, 0) ?? 0

    return (
        <div className="app-shell">
            <header className="topbar">
                <div className="brand-lockup">
                    <span className="brand-mark"><img src={grackleAvatar} alt=""/></span>
                    <span className="brand-name">PHOTO ID</span>
                </div>
                <span className="topbar-label">QUIZ BUILDER</span>
            </header>

            <main className="main-content">
                <p className="eyebrow">TRIP TO QUIZZES</p>
                <h1>Build daily quizzes from a trip</h1>
                <p className="intro">
                    Choose a trip itinerary and the folder of hotspot species data. The builder averages
                    frequencies, adds species named in the itinerary, sorts them taxonomically, and writes one
                    quiz file per day for the Photo ID runner.
                </p>

                <div className="steps">
                    <div className="step-row">
                        <span className="step-label">1. TRIP FILE</span>
                        <span className={tripPath ? 'step-value' : 'step-value placeholder'} title={tripPath}>
                            {tripPath ? fileName(tripPath) : 'No trip JSON chosen'}
                        </span>
                        <button className="secondary-button" onClick={pickTrip} disabled={busy}>Choose…</button>
                    </div>
                    <div className="step-row">
                        <span className="step-label">2. HOTSPOT DATA</span>
                        <span className={hotspotDir ? 'step-value' : 'step-value placeholder'} title={hotspotDir}>
                            {hotspotDir || 'No folder chosen'}
                            <span className="step-hint">Folder of L12345.json files, e.g. the legacy .cache/hotspots.</span>
                        </span>
                        <button className="secondary-button" disabled={busy}
                                onClick={() => pickFolder('Choose hotspot data folder', setHotspotDir)}>Choose…
                        </button>
                    </div>
                    <div className="step-row">
                        <span className="step-label">3. OUTPUT FOLDER</span>
                        <span className={outputDir ? 'step-value' : 'step-value placeholder'} title={outputDir}>
                            {outputDir || 'Defaults to generated_quizzes beside the trip file'}
                        </span>
                        <button className="secondary-button" disabled={busy}
                                onClick={() => pickFolder('Choose output folder', setOutputDir)}>Choose…
                        </button>
                    </div>
                </div>

                <div className="actions">
                    <button className="primary-button" onClick={onPreview} disabled={!ready || busy}>
                        {busy ? 'Working…' : 'Preview quizzes'}
                    </button>
                    {preview && (
                        <button className="accent-button" onClick={onWrite} disabled={busy}>
                            Write {preview.quizzes.length} quiz files
                        </button>
                    )}
                </div>

                {error && <div className="message error" role="alert">{error}</div>}
                {written && (
                    <div className="message success" role="status">
                        Wrote {written.length} quiz files to {preview?.outputDir}. Open them in the runner.
                    </div>
                )}

                {preview && (
                    <section className="results">
                        <div className="results-heading">
                            <div>
                                <p className="section-label">PREVIEW</p>
                                <h2>{preview.tripName || 'Untitled trip'}</h2>
                            </div>
                            <span className="summary-label">
                                {preview.quizzes.length} DAYS · {totalSpecies} SPECIES
                            </span>
                        </div>
                        <p className="output-path">Output: {preview.outputDir}</p>

                        {preview.warnings.length > 0 && (
                            <div className="message notice">
                                {preview.warnings.length} warning{preview.warnings.length === 1 ? '' : 's'}
                                <ul>{preview.warnings.map((w, i) => <li key={i}>{w}</li>)}</ul>
                            </div>
                        )}

                        <ul className="day-list">
                            {preview.quizzes.map((q, i) => (
                                <li className="day-item" key={q.fileName}>
                                    <button className="day-toggle" aria-expanded={open === i}
                                            onClick={() => setOpen(open === i ? null : i)}>
                                        <span className="day-number">{q.day}</span>
                                        <span className="day-title">
                                            <strong>{q.quiz.notes || 'Untitled day'}</strong>
                                            <span>{q.fileName}</span>
                                        </span>
                                        <span className="day-count">{q.quiz.species.length} species</span>
                                    </button>
                                    {open === i && (
                                        <ul className="species-table">
                                            {q.quiz.species.map(s => (
                                                <li key={s.comName}>
                                                    <span>{s.comName}</span>
                                                    {s.frequency != null
                                                        ? <span className="freq">{s.frequency}%</span>
                                                        : <span className="mentioned">mentioned</span>}
                                                </li>
                                            ))}
                                        </ul>
                                    )}
                                </li>
                            ))}
                        </ul>
                    </section>
                )}
            </main>
        </div>
    )
}
