import { type ParentComponent, createRenderEffect } from 'solid-js';
import { useLocation } from '@solidjs/router';

export const PageTransition: ParentComponent = (props) => {
  const location = useLocation();
  let ref!: HTMLDivElement;

  createRenderEffect(() => {
    location.pathname; // subscribe to route changes
    if (ref) {
      ref.style.animation = 'none';
      void ref.offsetHeight; // force reflow
      ref.style.animation = '';
    }
  });

  return (
    <div
      ref={ref!}
      class="page-transition flex flex-1 flex-col min-w-0 animate-[page-enter_250ms_ease-out_both] print:block print:animate-none"
      onAnimationEnd={() => {
        if (ref) {
          ref.style.animation = 'none';
        }
      }}
    >
      {props.children}
    </div>
  );
};
