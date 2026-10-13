import type { ComponentProps } from 'react'
import './Button.css'

// go: Start crawl and Resume crawl (green fill). stop: Stop crawl (red fill). pause: Pause
// (yellow outline). primary: the other main actions, such as Save and Check redirects
// (violet fill). secondary: everything else. Links that act as buttons (export downloads)
// use the same classes: className="btn btn-secondary".
export type ButtonVariant = 'go' | 'stop' | 'pause' | 'primary' | 'secondary'

type ButtonProps = ComponentProps<'button'> & { variant?: ButtonVariant }

export default function Button({ variant = 'secondary', className, type = 'button', ...rest }: ButtonProps) {
  const classes = className ? `btn btn-${variant} ${className}` : `btn btn-${variant}`
  return <button type={type} className={classes} {...rest} />
}
