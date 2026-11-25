import {
	createContext,
	useCallback,
	useContext,
	useEffect,
	useRef,
	useState,
	type ReactNode,
} from 'react'

export type ToastVariant = 'success' | 'error' | 'warning' | 'info'

type ToastItem = {
	id: number
	message: string
	variant: ToastVariant
}

type ToastContextValue = {
	notify: (message: string, variant?: ToastVariant) => void
	dismiss: (id: number) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

function ToastViewport({
	items,
	onDismiss,
}: {
	items: ToastItem[]
	onDismiss: (id: number) => void
}) {
	return (
		<div className='toast-viewport' aria-label='Notifications'>
			{items.map(item => (
				<div
					key={item.id}
					className={`toast ${item.variant}`}
					role={item.variant === 'error' ? 'alert' : 'status'}
					aria-live={item.variant === 'error' ? 'assertive' : 'polite'}
				>
					<span className='toast-indicator' aria-hidden='true'>
						{item.variant === 'success' ? '✓' : '!'}
					</span>

					<p>{item.message}</p>

					<button
						type='button'
						className='toast-dismiss'
						onClick={() => onDismiss(item.id)}
						aria-label='Dismiss notification'
					>
						×
					</button>
				</div>
			))}
		</div>
	)
}

export function ToastProvider({ children }: { children: ReactNode }) {
	const [items, setItems] = useState<ToastItem[]>([])
	const nextId = useRef(1)
	const timers = useRef(new Map<number, number>())

	const dismiss = useCallback((id: number) => {
		const timer = timers.current.get(id)

		if (timer !== undefined) {
			window.clearTimeout(timer)
			timers.current.delete(id)
		}

		setItems(current => current.filter(item => item.id !== id))
	}, [])

	const notify = useCallback(
		(message: string, variant: ToastVariant = 'info') => {
			const id = nextId.current++

			setItems(current => [...current, { id, message, variant }].slice(-4))

			const timer = window.setTimeout(() => {
				timers.current.delete(id)
				setItems(current => current.filter(item => item.id !== id))
			}, 5000)

			timers.current.set(id, timer)
		},
		[]
	)

	useEffect(() => {
		return () => {
			for (const timer of timers.current.values()) {
				window.clearTimeout(timer)
			}

			timers.current.clear()
		}
	}, [])

	return (
		<ToastContext.Provider value={{ notify, dismiss }}>
			{children}
			<ToastViewport items={items} onDismiss={dismiss} />
		</ToastContext.Provider>
	)
}

export function useToast() {
	const context = useContext(ToastContext)

	if (!context) {
		throw new Error('useToast must be used inside ToastProvider')
	}

	return context
}
