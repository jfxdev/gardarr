import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { taskMetadataService } from "@/services/taskMetadata";
import { useIsPortraitMobileOrTablet } from "@/hooks/use-portrait-mobile-tablet";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Calendar, Check, Image as ImageIcon, Loader2, Search, X } from "lucide-react";
import type { Category } from "@/types/category";
import type { MetadataProviderSearchResult, Task, TaskMetadata } from "@/types/torrent";

interface TaskMetadataSearchSheetProps {
  isOpen: boolean;
  onClose: () => void;
  task: Task | null;
  category: Category | null;
  onApplied: (metadata?: TaskMetadata) => Promise<void> | void;
}

const getReleaseDateTimestamp = (releaseDate?: string | null): number => {
  if (!releaseDate) {
    return 0;
  }

  const timestamp = new Date(releaseDate).getTime();
  return Number.isNaN(timestamp) ? 0 : timestamp;
};

const sortMetadataResultsByReleaseDate = (
  results: MetadataProviderSearchResult[]
): MetadataProviderSearchResult[] =>
  [...results].sort(
    (left, right) =>
      getReleaseDateTimestamp(right.release_date) - getReleaseDateTimestamp(left.release_date)
  );

export function TaskMetadataSearchSheet({
  isOpen,
  onClose,
  task,
  category,
  onApplied,
}: TaskMetadataSearchSheetProps) {
  const { t } = useTranslation();
  const isPortraitMobileOrTablet = useIsPortraitMobileOrTablet();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<MetadataProviderSearchResult[]>([]);
  const [selectedResultId, setSelectedResultId] = useState<string | null>(null);
  const [isSearching, setIsSearching] = useState(false);
  const [isApplying, setIsApplying] = useState(false);
  const [isProviderActive, setIsProviderActive] = useState(false);
  const [statusMessage, setStatusMessage] = useState("");
  const [searchError, setSearchError] = useState("");
  const [applyError, setApplyError] = useState("");

  const provider = category?.metadata_source || "none";
  const isSupportedProvider = provider === "tgdb" || provider === "tmdb";
  const initialQuery = useMemo(
    () => task?.metadata?.name?.trim() || task?.name?.trim() || "",
    [task?.metadata?.name, task?.name]
  );
  const selectedResult = useMemo(
    () => results.find((result) => result.id === selectedResultId) ?? null,
    [results, selectedResultId]
  );
  const isBusy = isSearching || isApplying;

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    setQuery(initialQuery);
    setResults([]);
    setSelectedResultId(null);
    setIsSearching(false);
    setIsApplying(false);
    setIsProviderActive(false);
    setStatusMessage("");
    setSearchError("");
    setApplyError("");
  }, [initialQuery, isOpen, task?.id]);

  const runSearch = useCallback(async (searchQuery: string, auto = false) => {
    const trimmedQuery = searchQuery.trim();

    if (!isSupportedProvider) {
      return;
    }

    if (trimmedQuery.length < 2) {
      setResults([]);
      setSelectedResultId(null);
      setSearchError(t("torrents.addModal.metadata.queryRequired"));
      return;
    }

    setIsSearching(true);
    setQuery(trimmedQuery);
    setSearchError("");
    setApplyError("");

    try {
      const response = await taskMetadataService.searchProvider(provider, trimmedQuery, auto);

      if (response.error) {
        setResults([]);
        setSelectedResultId(null);
        setSearchError(response.error || t(`${provider}.errors.searchFailed`));
        return;
      }

      const nextResults = sortMetadataResultsByReleaseDate(response.data || []);
      setResults(nextResults);
      setSelectedResultId(nextResults[0]?.id || null);

      if (nextResults.length === 0) {
        setSearchError(t("torrents.addModal.metadata.noResults"));
      }
    } catch (error) {
      const message = error instanceof Error
        ? error.message
        : t(`${provider}.errors.searchFailed`);
      setResults([]);
      setSelectedResultId(null);
      setSearchError(message);
      return;
    } finally {
      setIsSearching(false);
    }
  }, [isSupportedProvider, provider, t]);

  useEffect(() => {
    if (!isOpen || !task) {
      return;
    }

    if (!isSupportedProvider) {
      setStatusMessage(t("torrents.addModal.metadata.noProvider"));
      return;
    }

    let mounted = true;

    const loadProviderStatus = async () => {
      const response = await taskMetadataService.getProviderStatus(provider);
      if (!mounted) {
        return;
      }

      if (response.data?.active) {
        setIsProviderActive(true);
        setStatusMessage("");

        if (initialQuery.trim().length >= 2) {
          void runSearch(initialQuery, true);
        }
        return;
      }

      setIsProviderActive(false);
      setStatusMessage(response.error || t(`torrents.addModal.metadata.${provider}Inactive`));
    };

    void loadProviderStatus();

    return () => {
      mounted = false;
    };
  }, [initialQuery, isOpen, isSupportedProvider, provider, runSearch, t, task]);

  const handleApply = async () => {
    if (!task || !selectedResult || !isSupportedProvider) {
      return;
    }

    setIsApplying(true);
    setApplyError("");

    try {
      const response = await taskMetadataService.applyProvider(task.hash, provider, {
        id: selectedResult.id,
        title: selectedResult.title,
        release_date: selectedResult.release_date,
        description: selectedResult.description,
        image_id: selectedResult.image_id,
      });

      if (response.error) {
        const message = response.error || t(`${provider}.errors.applyFailed`);
        setApplyError(message);
        toast.error(message);
        return;
      }

      toast.success(t(`${provider}.success.applied`));
      await onApplied(response.data);
      onClose();
    } catch (err) {
      const message = err instanceof Error ? err.message : t(`${provider}.errors.applyFailed`);
      setApplyError(message);
      toast.error(message);
      return;
    } finally {
      setIsApplying(false);
    }
  };

  if (!task) {
    return null;
  }

  return (
    <Sheet open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <SheetContent
        side={isPortraitMobileOrTablet ? "bottom" : "right"}
        hideClose
        className="flex h-[calc(100dvh-0.5rem)] flex-col gap-0 rounded-t-2xl border-x border-t p-0 sm:h-full sm:max-w-2xl sm:rounded-none sm:border-x-0 sm:border-t-0"
      >
        <div className="flex items-center justify-between border-b bg-card p-4 sm:p-6">
          <div>
            <h2 className="text-2xl font-bold">{t("torrents.addModal.metadata.searchButton")}</h2>
            <p className="text-sm text-muted-foreground">{t("torrents.addModal.metadata.description")}</p>
          </div>
          <Button variant="ghost" size="icon" onClick={onClose} className="h-8 w-8">
            <X className="h-4 w-4" />
          </Button>
        </div>

        <ScrollArea className="h-full min-h-0">
          <div className="space-y-6 p-4 sm:p-6">
            <div className="space-y-2">
              <Label htmlFor="metadata-search-query">{t("torrents.addModal.metadata.fields.name")}</Label>
              <div className="flex flex-col gap-3 sm:flex-row">
                <div className="relative flex-1">
                  <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    id="metadata-search-query"
                    value={query}
                    onChange={(event) => {
                      setQuery(event.target.value);
                      setSearchError("");
                    }}
                    placeholder={t("torrents.addModal.metadata.searchPlaceholder")}
                    className="pl-9"
                    disabled={!isSupportedProvider || !isProviderActive || isBusy}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        void runSearch(query);
                      }
                    }}
                  />
                </div>
                <Button
                  type="button"
                  onClick={() => void runSearch(query)}
                  disabled={!isSupportedProvider || !isProviderActive || isBusy}
                  className="w-full sm:w-auto"
                >
                  {isSearching ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t("torrents.addModal.metadata.searching")}
                    </>
                  ) : (
                    t("torrents.addModal.metadata.searchButton")
                  )}
                </Button>
              </div>
            </div>

            {statusMessage && (
              <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                {statusMessage}
              </div>
            )}

            {(searchError || applyError) && (
              <div className="space-y-1 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2">
                <p className="text-sm text-destructive">{applyError || searchError}</p>
                {isSupportedProvider && (
                  <p className="text-xs text-muted-foreground">
                    {t("torrents.addModal.metadata.searchSuggestion")}
                  </p>
                )}
              </div>
            )}

            {isSupportedProvider && results.length > 0 && (
              <div className="space-y-3">
                <div>
                  <h3 className="font-medium">{t("torrents.addModal.metadata.resultsLabel")}</h3>
                  <p className="text-sm text-muted-foreground">{task.metadata?.name || task.name}</p>
                </div>

                <div className="grid gap-3">
                  {results.map((result) => {
                    const isSelected = selectedResultId === result.id;
                    const previewUrl = taskMetadataService.getProviderImagePreviewUrl(provider, result.image_id);

                    return (
                      <button
                        key={result.id}
                        type="button"
                        onClick={() => setSelectedResultId(result.id)}
                        disabled={isBusy}
                        className={`flex w-full items-start gap-4 rounded-lg border p-3 text-left transition-colors hover:border-primary/40 hover:bg-accent/30 ${
                          isSelected ? "border-primary bg-primary/5" : ""
                        }`}
                      >
                        <div className="h-20 w-20 shrink-0 overflow-hidden rounded-md border bg-muted/40">
                          {previewUrl ? (
                            <img
                              src={previewUrl}
                              alt={result.title}
                              className="h-full w-full object-cover"
                            />
                          ) : (
                            <div className="flex h-full w-full items-center justify-center text-muted-foreground">
                              <ImageIcon className="h-5 w-5" />
                            </div>
                          )}
                        </div>

                        <div className="min-w-0 flex-1 space-y-2">
                          <div className="flex items-start justify-between gap-3">
                            <div className="min-w-0">
                              <p className="truncate font-medium">{result.title}</p>
                              <p className="flex items-center gap-2 text-xs text-muted-foreground">
                                <Calendar className="h-3.5 w-3.5" />
                                {result.release_date || t(`${provider}.search.unknownDate`)}
                              </p>
                            </div>
                            {isSelected && <Check className="mt-0.5 h-4 w-4 shrink-0 text-primary" />}
                          </div>

                          <p className="line-clamp-3 text-sm text-muted-foreground">
                            {result.description || t("torrents.addModal.metadata.descriptionPlaceholder")}
                          </p>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        </ScrollArea>

        <div className="flex flex-col-reverse gap-3 border-t bg-card px-4 py-4 sm:flex-row sm:justify-between sm:px-6">
          <Button type="button" variant="outline" onClick={onClose} disabled={isBusy}>
            {t("common.cancel")}
          </Button>
          <Button
            type="button"
            onClick={() => void handleApply()}
            disabled={!selectedResult || !isSupportedProvider || !isProviderActive || isBusy}
            className="w-full sm:w-auto"
          >
            {isApplying ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                {t("torrents.addModal.actions.finishing")}
              </>
            ) : (
              t("torrents.addModal.actions.finish")
            )}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  );
}
