type SpinnerProps = {
	size?: 'small' | 'medium'
}

export function Spinner({ size = 'small' }: SpinnerProps) {
	return <span className={`spinner spinner-${size}`} aria-hidden='true' />
}
