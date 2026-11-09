import { LOCALE } from '~/domain';

import { type Component } from 'solid-js';
import { useI18n } from '~/i18n';
import { IconLanguage, IconCheck } from '@tabler/icons-solidjs';
import { Button } from '~/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from '~/components/ui/dropdown-menu';

export const LanguageToggle: Component = () => {
  const { locale, setLocale, t } = useI18n();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        as={Button<"button">}
        variant="ghost"
        size="icon"
        class="size-9 rounded-md border border-input bg-background hover:bg-accent"
        aria-label={t("language.toggle")}
      >
        <span class="sr-only">{t("language.toggle")}</span>
        <IconLanguage class="size-4 text-foreground" />
      </DropdownMenuTrigger>
      <DropdownMenuContent class="min-w-[8.5rem]">
		<DropdownMenuItem onSelect={() => setLocale(LOCALE.ENGLISH)} class="cursor-pointer gap-2 flex items-center">
			<span class="flex-1">English</span>
			{locale() === LOCALE.ENGLISH && <IconCheck class="size-3.5 text-primary" />}
		</DropdownMenuItem>
		<DropdownMenuItem onSelect={() => setLocale(LOCALE.INDONESIAN)} class="cursor-pointer gap-2 flex items-center">
			<span class="flex-1">Bahasa Indonesia</span>
			{locale() === LOCALE.INDONESIAN && <IconCheck class="size-3.5 text-primary" />}
		</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
