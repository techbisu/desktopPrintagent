import { useEffect, useState } from 'react'
import { GetQueue, RetryJob, ConfirmJob } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

// Mirrors internal/queue.PrintJob on the Go side.
export interface PrintJob {
  id: string
  serviceCode: string
  filename: string
  fileUrl: string
  fileType: string
  pages: number
  copies: number
  isColor: boolean
  isDuplex: boolean
  totalAmount: number
  status: 'QUEUED' | 'DOWNLOADING' | 'PRINTING' | 'COMPLETED' | 'FAILED'
  printerAssigned: string
  error?: string
  pendingConfirmation?: boolean
}

export default function Queue() {
  const [jobs, setJobs] = useState<PrintJob[]>([])
  const [busyId, setBusyId] = useState<string | null>(null)

  useEffect(() => {
    GetQueue().then(setJobs)

    const unsubscribe = EventsOn('job:status', (updated: PrintJob) => {
      setJobs((prev) => {
        const idx = prev.findIndex((j) => j.id === updated.id)
        if (idx === -1) return [...prev, updated]
        const next = [...prev]
        next[idx] = updated
        return next
      })
    })

    return () => unsubscribe()
  }, [])

  async function handleRetry(id: string) {
    setBusyId(id)
    try {
      await RetryJob(id)
    } finally {
      setBusyId(null)
    }
  }

  async function handleConfirm(id: string) {
    setBusyId(id)
    try {
      await ConfirmJob(id)
    } finally {
      setBusyId(null)
    }
  }

  if (jobs.length === 0) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-slate-400">
        No print jobs yet. Waiting for customers to scan the counter QR code…
      </div>
    )
  }

  return (
    <div className="overflow-hidden rounded-lg border border-slate-200 bg-white">
      <table className="w-full text-left text-sm">
        <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
          <tr>
            <th className="px-4 py-2 font-medium">Filename</th>
            <th className="px-4 py-2 font-medium">Pages</th>
            <th className="px-4 py-2 font-medium">Type</th>
            <th className="px-4 py-2 font-medium">Price</th>
            <th className="px-4 py-2 font-medium">Printer</th>
            <th className="px-4 py-2 font-medium">Status</th>
            <th className="px-4 py-2 font-medium" />
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {jobs
            .slice()
            .reverse()
            .map((job) => (
              <tr key={job.id}>
                <td className="max-w-[180px] truncate px-4 py-2">{job.filename}</td>
                <td className="px-4 py-2">
                  {job.pages} × {job.copies}
                </td>
                <td className="px-4 py-2">
                  <ColorBadge isColor={job.isColor} />
                </td>
                <td className="px-4 py-2">₹{job.totalAmount.toFixed(2)}</td>
                <td className="px-4 py-2 text-slate-500">{job.printerAssigned || '—'}</td>
                <td className="px-4 py-2">
                  <StatusBadge status={job.status} error={job.error} pending={job.pendingConfirmation} />
                </td>
                <td className="px-4 py-2 text-right">
                  {job.status === 'FAILED' && (
                    <button
                      onClick={() => handleRetry(job.id)}
                      disabled={busyId === job.id}
                      className="rounded-md bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-200 disabled:opacity-50"
                    >
                      {busyId === job.id ? 'Retrying…' : 'Retry'}
                    </button>
                  )}
                  {job.status === 'QUEUED' && job.pendingConfirmation && (
                    <button
                      onClick={() => handleConfirm(job.id)}
                      disabled={busyId === job.id}
                      className="rounded-md bg-brand-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-brand-700 disabled:opacity-50"
                    >
                      {busyId === job.id ? 'Printing…' : 'Print Now'}
                    </button>
                  )}
                </td>
              </tr>
            ))}
        </tbody>
      </table>
    </div>
  )
}

function ColorBadge({ isColor }: { isColor: boolean }) {
  return (
    <span
      className={`rounded-full px-2 py-0.5 text-xs font-medium ${
        isColor ? 'bg-fuchsia-100 text-fuchsia-700' : 'bg-slate-100 text-slate-600'
      }`}
    >
      {isColor ? 'Color' : 'B&W'}
    </span>
  )
}

const STATUS_STYLES: Record<PrintJob['status'], string> = {
  QUEUED: 'bg-slate-100 text-slate-600',
  DOWNLOADING: 'bg-amber-100 text-amber-700',
  PRINTING: 'bg-blue-100 text-blue-700',
  COMPLETED: 'bg-emerald-100 text-emerald-700',
  FAILED: 'bg-red-100 text-red-700',
}

function StatusBadge({
  status,
  error,
  pending,
}: {
  status: PrintJob['status']
  error?: string
  pending?: boolean
}) {
  const label = status === 'QUEUED' && pending ? 'AWAITING CONFIRMATION' : status
  return (
    <span
      title={error}
      className={`rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_STYLES[status]}`}
    >
      {label}
    </span>
  )
}
