import type { ValidComponent } from "solid-js"
import { splitProps } from "solid-js"

import type { PolymorphicProps } from "@kobalte/core/polymorphic"
import * as TabsPrimitive from "@kobalte/core/tabs"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "~/lib/utils"

const Tabs = TabsPrimitive.Root

const tabsListVariants = cva(
  "inline-flex h-9 items-center justify-center rounded-lg bg-muted p-1 text-muted-foreground"
)

type TabsListProps<T extends ValidComponent = "div"> = TabsPrimitive.TabsListProps<T> &
  VariantProps<typeof tabsListVariants> & {
    class?: string | undefined
  }

const TabsList = <T extends ValidComponent = "div">(
  props: PolymorphicProps<T, TabsListProps<T>>
) => {
  const [local, others] = splitProps(props as TabsListProps, ["class"])
  return (
    <TabsPrimitive.List
      class={cn(tabsListVariants(), local.class)}
      {...others}
    />
  )
}

const tabsTriggerVariants = cva(
  "inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1 text-sm font-medium ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 data-[selected]:bg-background data-[selected]:text-foreground data-[selected]:shadow"
)

type TabsTriggerProps<T extends ValidComponent = "button"> = TabsPrimitive.TabsTriggerProps<T> &
  VariantProps<typeof tabsTriggerVariants> & {
    class?: string | undefined
  }

const TabsTrigger = <T extends ValidComponent = "button">(
  props: PolymorphicProps<T, TabsTriggerProps<T>>
) => {
  const [local, others] = splitProps(props as TabsTriggerProps, ["class"])
  return (
    <TabsPrimitive.Trigger
      class={cn(tabsTriggerVariants(), local.class)}
      {...others}
    />
  )
}

const tabsContentVariants = cva(
  "mt-2 ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
)

type TabsContentProps<T extends ValidComponent = "div"> = TabsPrimitive.TabsContentProps<T> &
  VariantProps<typeof tabsContentVariants> & {
    class?: string | undefined
  }

const TabsContent = <T extends ValidComponent = "div">(
  props: PolymorphicProps<T, TabsContentProps<T>>
) => {
  const [local, others] = splitProps(props as TabsContentProps, ["class"])
  return (
    <TabsPrimitive.Content
      class={cn(tabsContentVariants(), local.class)}
      {...others}
    />
  )
}

const TabsIndicator = TabsPrimitive.Indicator

export { Tabs, TabsList, TabsTrigger, TabsContent, TabsIndicator }
