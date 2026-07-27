import type { LaunchParameter } from "../../api/sessions";
import { profileParameterID } from "../../lib/tools";

interface ProfileArgsProps {
  parameters: readonly LaunchParameter[];
  values: Readonly<Record<string, unknown>>;
  disabled: boolean;
  onChange: (id: string, value: string | number | boolean) => void;
}

function parameterLabel(parameter: LaunchParameter): string {
  return parameter.label || parameter.name;
}

function isAtInitial(parameter: LaunchParameter, value: unknown): boolean {
  return parameter.default !== undefined && value === parameter.default;
}

function SegmentedArg({
  parameter,
  value,
  disabled,
  onChange,
}: {
  parameter: Extract<LaunchParameter, { type: "enum" | "boolean" }>;
  value: unknown;
  disabled: boolean;
  onChange: (value: string | boolean) => void;
}) {
  const choices =
    parameter.type === "boolean"
      ? [
          { value: true, label: "On" },
          { value: false, label: "Off" },
        ]
      : parameter.values.map((choice) => ({ value: choice, label: choice }));
  const label = parameterLabel(parameter);
  return (
    <div
      className="palette-profile-arg"
      data-testid={`profile-arg-${parameter.name}`}
      data-at-initial={isAtInitial(parameter, value)}
    >
      <span className="palette-profile-arg__label" id={`profile-arg-${parameter.name}-label`}>
        {label}
      </span>
      <div
        className="palette-profile-arg__segments"
        role="radiogroup"
        aria-labelledby={`profile-arg-${parameter.name}-label`}
      >
        {choices.map((choice) => {
          const selected = value === choice.value;
          const initial = parameter.default !== undefined && parameter.default === choice.value;
          return (
            <button
              key={String(choice.value)}
              type="button"
              // biome-ignore lint/a11y/useSemanticElements: segmented controls use buttons so each option can carry the non-textual initial-value marker without rendering native radio chrome
              role="radio"
              aria-checked={selected}
              aria-label={`${choice.label}${initial ? ", initial value" : ""}`}
              className="palette-profile-arg__segment"
              data-initial={initial}
              disabled={disabled}
              onClick={() => onChange(choice.value)}
            >
              {choice.label}
            </button>
          );
        })}
      </div>
      {parameter.default !== undefined && !isAtInitial(parameter, value) && (
        <button
          type="button"
          className="palette-profile-arg__reset"
          aria-label={`Reset ${label} to initial value`}
          title="Reset to initial value"
          disabled={disabled}
          onClick={() => onChange(parameter.default as string | boolean)}
        >
          ↺
        </button>
      )}
    </div>
  );
}

function TextArg({
  parameter,
  value,
  disabled,
  onChange,
}: {
  parameter: Extract<LaunchParameter, { type: "string" | "number" }>;
  value: unknown;
  disabled: boolean;
  onChange: (value: string | number) => void;
}) {
  const label = parameterLabel(parameter);
  const atInitial = isAtInitial(parameter, value);
  const initial = parameter.default;
  return (
    <div
      className="palette-profile-arg"
      data-testid={`profile-arg-${parameter.name}`}
      data-at-initial={atInitial}
    >
      <label className="palette-profile-arg__label" htmlFor={`profile-arg-${parameter.name}`}>
        {label}
      </label>
      <div className="palette-profile-arg__input-wrap" data-initial={atInitial}>
        <input
          id={`profile-arg-${parameter.name}`}
          className="palette-profile-arg__input"
          type={parameter.type === "number" ? "number" : "text"}
          value={value === undefined ? "" : String(value)}
          min={parameter.type === "number" ? parameter.min : undefined}
          max={parameter.type === "number" ? parameter.max : undefined}
          pattern={parameter.type === "string" ? parameter.pattern : undefined}
          disabled={disabled}
          onChange={(event) => {
            if (parameter.type === "number") {
              const number = event.currentTarget.valueAsNumber;
              onChange(Number.isNaN(number) ? "" : number);
              return;
            }
            onChange(event.currentTarget.value);
          }}
        />
      </div>
      {initial !== undefined && !atInitial && (
        <button
          type="button"
          className="palette-profile-arg__reset"
          aria-label={`Reset ${label} to initial value`}
          title="Reset to initial value"
          disabled={disabled}
          onClick={() => onChange(initial)}
        >
          ↺
        </button>
      )}
    </div>
  );
}

export function ProfileArgs({
  parameters,
  values,
  disabled,
  onChange,
}: ProfileArgsProps): JSX.Element | null {
  if (parameters.length === 0) return null;
  return (
    <section className="palette-profile-args" aria-label="Command options">
      {parameters.map((parameter) => {
        const id = profileParameterID(parameter.name);
        const value = values[id];
        return parameter.type === "enum" || parameter.type === "boolean" ? (
          <SegmentedArg
            key={id}
            parameter={parameter}
            value={value}
            disabled={disabled}
            onChange={(next) => onChange(id, next)}
          />
        ) : (
          <TextArg
            key={id}
            parameter={parameter}
            value={value}
            disabled={disabled}
            onChange={(next) => onChange(id, next)}
          />
        );
      })}
    </section>
  );
}
