import { cn } from '~/lib/utils';

interface SpinnerProps {
  class?: string;
  size?: 'sm' | 'md' | 'lg';
}

export function Spinner(props: SpinnerProps) {
  const sizeClass = () => {
    switch (props.size) {
      case 'sm':
        return 'h-4 w-4 border-2';
      case 'lg':
        return 'h-8 w-8 border-[3px]';
      default:
        return 'h-5 w-5 border-2';
    }
  };

  return (
    <div
      class={cn(
        'animate-spin rounded-full border-primary border-t-transparent',
        sizeClass(),
        props.class
      )}
    />
  );
}
