import { useEffect, useState } from 'react'
import { GetConfig, SaveConfig, GetPrinters } from '../../wailsjs/go/main/App'

// Mirrors internal/config.Config on the Go side.
export interface AgentConfig {
  shopId: string
  authToken: string
  pusherAppKey: string
  pusherCluster: string
  pusherAuthUrl: string
  blackWhitePrinter: string
  colorPrinter: string
  silentAutoPrint: boolean
}

const emptyConfig: AgentConfig = {
  shopId: '',
  authToken: '',
  pusherAppKey: '',
  pusherCluster: '',
  pusherAuthUrl: '',
  blackWhitePrinter: '',
  colorPrinter: '',
  silentAutoPrint: true,
}

export default function Settings() {
  const [cfg, setCfg] = useState<AgentConfig>(emptyConfig)
  const [printers, setPrinters] = useState<string[]>([])
  const [status, setStatus] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle')
  const [errorMsg, setErrorMsg] = useState('')

  useEffect(() => {
    GetConfig().then((c: AgentConfig) => setCfg({ ...emptyConfig, ...c }))
    GetPrinters()
      .then(setPrinters)
      .catch(() => setPrinters([]))
  }, [])

  function update<K extends keyof AgentConfig>(key: K, value: AgentConfig[K]) {
    setCfg((prev) => ({ ...prev, [key]: value }))
  }

  async function handleSave() {
    setStatus('saving')
    setErrorMsg('')
    try {
      await SaveConfig(cfg)
      setStatus('saved')
      setTimeout(() => setStatus('idle'), 1800)
    } catch (err) {
      setStatus('error')
      setErrorMsg(String(err))
    }
  }

  return (
    <div className="mx-auto max-w-xl space-y-6">
      <Section title="Shop Credentials">
        <Field label="Shop ID">
          <input
            className="input"
            value={cfg.shopId}
            onChange={(e) => update('shopId', e.target.value)}
            placeholder="e.g. c9c1e1a2-..."
          />
        </Field>
        <Field label="Auth Token">
          <input
            type="password"
            className="input"
            value={cfg.authToken}
            onChange={(e) => update('authToken', e.target.value)}
            placeholder="Paste the token from your web dashboard"
          />
        </Field>
      </Section>

      <Section title="Realtime Connection">
        <Field label="Pusher App Key">
          <input
            className="input"
            value={cfg.pusherAppKey}
            onChange={(e) => update('pusherAppKey', e.target.value)}
          />
        </Field>
        <Field label="Pusher Cluster">
          <input
            className="input"
            value={cfg.pusherCluster}
            onChange={(e) => update('pusherCluster', e.target.value)}
            placeholder="e.g. ap2"
          />
        </Field>
        <Field label="Auth Endpoint URL">
          <input
            className="input"
            value={cfg.pusherAuthUrl}
            onChange={(e) => update('pusherAuthUrl', e.target.value)}
            placeholder="https://yourdomain.com/api/pusher/auth"
          />
        </Field>
      </Section>

      <Section title="Printers">
        <Field label="Black & White Printer">
          <select
            className="input"
            value={cfg.blackWhitePrinter}
            onChange={(e) => update('blackWhitePrinter', e.target.value)}
          >
            <option value="">Select a printer…</option>
            {printers.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Color Printer">
          <select
            className="input"
            value={cfg.colorPrinter}
            onChange={(e) => update('colorPrinter', e.target.value)}
          >
            <option value="">Select a printer…</option>
            {printers.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        </Field>
      </Section>

      <Section title="Printing Behavior">
        <label className="flex items-center justify-between rounded-lg border border-slate-200 bg-white px-4 py-3">
          <div>
            <p className="text-sm font-medium">Silent Auto-Print</p>
            <p className="text-xs text-slate-500">
              Print jobs the instant they arrive. Turn off to require manual confirmation per job.
            </p>
          </div>
          <input
            type="checkbox"
            checked={cfg.silentAutoPrint}
            onChange={(e) => update('silentAutoPrint', e.target.checked)}
            className="h-5 w-5 accent-brand-600"
          />
        </label>
      </Section>

      <div className="flex items-center gap-3">
        <button
          onClick={handleSave}
          disabled={status === 'saving'}
          className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-60"
        >
          {status === 'saving' ? 'Saving…' : 'Save Settings'}
        </button>
        {status === 'saved' && <span className="text-sm text-emerald-600">Saved.</span>}
        {status === 'error' && <span className="text-sm text-red-600">Error: {errorMsg}</span>}
      </div>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 className="text-xs font-semibold uppercase tracking-wide text-slate-400">{title}</h2>
      <div className="space-y-3">{children}</div>
    </section>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1">
      <span className="text-xs font-medium text-slate-600">{label}</span>
      {children}
    </label>
  )
}
