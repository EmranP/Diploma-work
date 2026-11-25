import type { ReactNode } from 'react'

type AlertVariant = 'error' | 'warning' | 'info'

type InlineAlertProps = {
	variant: AlertVariant
	title?: string
	children: ReactNode
	action?: {
		label: string
		onClick: () => void
	}
}

export function InlineAlert({
	variant,
	title,
	children,
	action,
}: InlineAlertProps) {
	return (
		<div
			className={`inline-alert ${variant}`}
			role={variant === 'error' ? 'alert' : 'status'}
		>
			<div className='inline-alert-content'>
				{title && <strong>{title}</strong>}
				<div>{children}</div>
			</div>

			{action && (
				<button
					type='button'
					className='inline-alert-action'
					onClick={action.onClick}
				>
					{action.label}
				</button>
			)}
		</div>
	)
}
