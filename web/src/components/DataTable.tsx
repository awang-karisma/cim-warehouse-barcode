import type { Component, ComponentProps } from "solid-js"
import { Table } from "~/components/ui/table"

export const DataTable: Component<ComponentProps<"table">> = (props) => {
  return (
    <div class="rounded-md border overflow-x-auto">
      <Table {...props} />
    </div>
  )
}
