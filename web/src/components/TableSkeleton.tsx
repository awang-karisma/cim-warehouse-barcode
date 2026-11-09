import { For, createMemo } from 'solid-js';
import { TableRow, TableCell } from '~/components/ui/table';
import { Skeleton } from '~/components/ui/skeleton';

interface TableSkeletonProps {
  columns: number;
  rows?: number;
}

const widths = ['w-3/4', 'w-1/2', 'w-5/6', 'w-2/3', 'w-4/5'];

export function TableSkeleton(props: TableSkeletonProps) {
  const rowList = createMemo(() => Array.from({ length: props.rows ?? 5 }));
  const colList = createMemo(() => Array.from({ length: props.columns }));

  return (
    <For each={rowList()}>
      {(_, rowIndex) => (
        <TableRow>
          <For each={colList()}>
            {(_, colIndex) => (
              <TableCell>
                <Skeleton class={`h-5 ${widths[(rowIndex() + colIndex()) % widths.length]}`} />
              </TableCell>
            )}
          </For>
        </TableRow>
      )}
    </For>
  );
}
