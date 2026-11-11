import type { JSX, ValidComponent } from "solid-js"
import { splitProps } from "solid-js"

import * as CheckboxPrimitive from "@kobalte/core/checkbox"
import type { PolymorphicProps } from "@kobalte/core/polymorphic"

import { cn } from "~/lib/utils"

type CheckboxProps<T extends ValidComponent = "div"> =
  CheckboxPrimitive.CheckboxRootProps<T> & {
    class?: string | undefined
  }

const Checkbox = <T extends ValidComponent = "div">(
  props: PolymorphicProps<T, CheckboxProps<T>>
) => {
  const [local, others] = splitProps(props as CheckboxProps, ["class"])
  return (
    <CheckboxPrimitive.Root
      class={cn("items-top group flex space-x-2", local.class)}
      {...others}
    />
  )
}

type CheckboxControlProps<T extends ValidComponent = "div"> =
  CheckboxPrimitive.CheckboxControlProps<T> & {
    class?: string | undefined
    children?: JSX.Element
  }

const CheckboxControl = <T extends ValidComponent = "div">(
  props: PolymorphicProps<T, CheckboxControlProps<T>>
) => {
  const [local, others] = splitProps(props as CheckboxControlProps, ["class", "children"])
  return (
    <>
      <CheckboxPrimitive.Input class="[&:focus-visible+div]:ring-2 [&:focus-visible+div]:ring-ring [&:focus-visible+div]:ring-offset-2 [&:focus-visible+div]:ring-offset-background" />
      <CheckboxPrimitive.Control
        class={cn(
          "size-4 shrink-0 rounded-sm border border-primary ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 data-[checked]:bg-primary data-[checked]:text-primary-foreground",
          local.class
        )}
        {...others}
      >
        <CheckboxPrimitive.Indicator class="flex items-center justify-center text-current">
          {local.children ?? (
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              class="size-4"
            >
              <polyline points="20 6 9 17 4 12" />
            </svg>
          )}
        </CheckboxPrimitive.Indicator>
      </CheckboxPrimitive.Control>
    </>
  )
}

type CheckboxLabelProps<T extends ValidComponent = "label"> =
  CheckboxPrimitive.CheckboxLabelProps<T> & {
    class?: string | undefined
  }

const CheckboxLabel = <T extends ValidComponent = "label">(
  props: PolymorphicProps<T, CheckboxLabelProps<T>>
) => {
  const [local, others] = splitProps(props as CheckboxLabelProps, ["class"])
  return (
    <CheckboxPrimitive.Label
      class={cn(
        "text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70",
        local.class
      )}
      {...others}
    />
  )
}

const CheckboxDescription = CheckboxPrimitive.Description
const CheckboxErrorMessage = CheckboxPrimitive.ErrorMessage

export {
  Checkbox,
  CheckboxControl,
  CheckboxLabel,
  CheckboxDescription,
  CheckboxErrorMessage,
}
