import {
  useId,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from "react";

import { cn } from "../../lib/cn";

const CONTROL =
  "block w-full rounded-md border-0 bg-white px-3 py-2 text-sm text-slate-900 ring-1 ring-inset " +
  "placeholder:text-slate-400 focus:ring-2 focus:ring-inset disabled:bg-slate-50 disabled:text-slate-500";

function controlClass(invalid: boolean, className?: string) {
  return cn(
    CONTROL,
    invalid ? "ring-red-400 focus:ring-red-500" : "ring-slate-300 focus:ring-blue-600",
    className,
  );
}

interface FieldChromeProps {
  label?: ReactNode;
  /** Validation message; also marks the control aria-invalid. */
  error?: string;
  hint?: ReactNode;
}

/** Label + control + hint/error, with ids wired for accessibility. */
function FieldChrome({
  id,
  label,
  error,
  hint,
  required,
  children,
}: FieldChromeProps & { id: string; required?: boolean; children: ReactNode }) {
  return (
    <div className="space-y-1">
      {label && (
        <label htmlFor={id} className="block text-sm font-medium text-slate-700">
          {label}
          {required && <span className="ml-0.5 text-red-600">*</span>}
        </label>
      )}
      {children}
      {error ? (
        <p id={`${id}-error`} className="text-xs text-red-600">
          {error}
        </p>
      ) : hint ? (
        <p id={`${id}-hint`} className="text-xs text-slate-500">
          {hint}
        </p>
      ) : null}
    </div>
  );
}

function describedBy(id: string, error?: string, hint?: ReactNode) {
  if (error) return `${id}-error`;
  if (hint) return `${id}-hint`;
  return undefined;
}

export interface InputProps extends InputHTMLAttributes<HTMLInputElement>, FieldChromeProps {}

export function Input({ label, error, hint, id, className, required, ...props }: InputProps) {
  const autoId = useId();
  const fieldId = id ?? autoId;
  return (
    <FieldChrome id={fieldId} label={label} error={error} hint={hint} required={required}>
      <input
        id={fieldId}
        required={required}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy(fieldId, error, hint)}
        className={controlClass(Boolean(error), className)}
        {...props}
      />
    </FieldChrome>
  );
}

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement>, FieldChromeProps {}

export function Select({
  label,
  error,
  hint,
  id,
  className,
  required,
  children,
  ...props
}: SelectProps) {
  const autoId = useId();
  const fieldId = id ?? autoId;
  return (
    <FieldChrome id={fieldId} label={label} error={error} hint={hint} required={required}>
      <select
        id={fieldId}
        required={required}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy(fieldId, error, hint)}
        className={controlClass(Boolean(error), cn("pr-8", className))}
        {...props}
      >
        {children}
      </select>
    </FieldChrome>
  );
}

export interface TextareaProps
  extends TextareaHTMLAttributes<HTMLTextAreaElement>, FieldChromeProps {}

export function Textarea({ label, error, hint, id, className, required, ...props }: TextareaProps) {
  const autoId = useId();
  const fieldId = id ?? autoId;
  return (
    <FieldChrome id={fieldId} label={label} error={error} hint={hint} required={required}>
      <textarea
        id={fieldId}
        required={required}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy(fieldId, error, hint)}
        className={controlClass(Boolean(error), className)}
        {...props}
      />
    </FieldChrome>
  );
}
