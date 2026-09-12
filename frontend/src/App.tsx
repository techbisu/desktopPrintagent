import { useState } from 'react'
import Settings from './components/Settings'
import Queue from './components/Queue'

type Tab = 'queue' | 'settings'

export default function App() {
  const [tab, setTab] = useState<Tab>('queue')

  return (
    <div className="flex h-screen w-screen flex-col bg-slate-50 text-slate-900">
      <header className="flex items-center justify-between border-b border-slate-200 bg-white px-5 py-3">
        <div className="flex items-center gap-2">
          <div className="h-2.5 w-2.5 rounded-full bg-emerald-500" />
          <h1 className="text-sm font-semibold tracking-tight">SmartPrint Agent</h1>
        </div>
        <nav className="flex gap-1 rounded-lg bg-slate-100 p-1">
          <TabButton active={tab === 'queue'} onClick={() => setTab('queue')}>
            Live Queue
          </TabButton>
          <TabButton active={tab === 'settings'} onClick={() => setTab('settings')}>
            Settings
          </TabButton>
        </nav>
      </header>

      <main className="flex-1 overflow-auto p-5">
        {tab === 'queue' ? <Queue /> : <Settings />}
      </main>
    </div>
  )
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
        active ? 'bg-white text-brand-700 shadow-sm' : 'text-slate-500 hover:text-slate-700'
      }`}
    >
      {children}
    </button>
  )
}
