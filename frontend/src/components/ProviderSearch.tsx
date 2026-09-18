import { useState, useCallback, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Search, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { taskMetadataService } from "@/services/taskMetadata";
import { useTranslation } from "react-i18next";
import type { TaskMetadata } from "@/types/torrent";

export type MetadataProvider = "tgdb" | "tmdb";

interface ProviderSearchResult {
  id: string;
  title: string;
  release_date?: string;
  description?: string;
  image_id?: string;
  image_url?: string;
}

interface ProviderSearchProps {
  provider: MetadataProvider;
  taskHash: string;
  initialQuery: string;
  onSelect: (metadata: TaskMetadata) => void;
  onCancel: () => void;
}

export function ProviderSearch({ provider, taskHash, initialQuery, onSelect, onCancel }: ProviderSearchProps) {
  const { t } = useTranslation();
  const [query, setQuery] = useState(initialQuery);
  const [isSearching, setIsSearching] = useState(false);
  const [results, setResults] = useState<ProviderSearchResult[]>([]);
  const [isApplying, setIsApplying] = useState(false);
  const [applyingResultId, setApplyingResultId] = useState<string | null>(null);

  useEffect(() => {
    setQuery((currentQuery) =>
      currentQuery === initialQuery ? currentQuery : initialQuery
    );
  }, [initialQuery]);

  // auto=true tells the backend to run the query through release-name
  // parsing first (needed for the raw torrent name on the initial
  // auto-search); a manually typed/edited re-search must be sent verbatim,
  // or the parser can silently drop words the user intentionally typed.
  const handleSearch = useCallback(async (searchQuery: string, auto = false) => {
    const trimmedQuery = searchQuery.trim();
    if (!trimmedQuery) return;

    setIsSearching(true);
    setQuery((currentQuery) =>
      currentQuery === trimmedQuery ? currentQuery : trimmedQuery
    );
    try {
      const autoParam = auto ? "&auto=true" : "";
      const response = await api.get<ProviderSearchResult[]>(`/tasks/metadata/${taskHash}/providers/${provider}/search?q=${encodeURIComponent(trimmedQuery)}${autoParam}`);
      setResults(response.data || []);
    } catch {
      toast.error(t(`${provider}.errors.searchFailed`));
    } finally {
      setIsSearching(false);
    }
  }, [taskHash, provider, t]);

  // Auto-search on open using the torrent's own name, so the tab isn't a
  // blank results grid behind a pre-filled input the user has to re-submit.
  useEffect(() => {
    handleSearch(initialQuery, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleApply = async (result: ProviderSearchResult) => {
    if (isApplying) {
      return;
    }

    setIsApplying(true);
    setApplyingResultId(result.id);
    try {
      const response = await api.post<TaskMetadata>(`/tasks/metadata/${taskHash}/providers/${provider}`, {
        id: result.id,
        title: result.title,
        release_date: result.release_date,
        description: result.description,
        image_id: result.image_id,
      });
      if (response.error || !response.data) {
        toast.error(response.error || t(`${provider}.errors.applyFailed`));
        return;
      }

      toast.success(t(`${provider}.success.applied`));
      onSelect(response.data);
    } catch {
      toast.error(t(`${provider}.errors.applyFailed`));
    } finally {
      setIsApplying(false);
      setApplyingResultId(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t(`${provider}.search.placeholder`)}
          aria-label={`Search ${provider.toUpperCase()}`}
          onKeyDown={(e) => e.key === "Enter" && handleSearch(query)}
        />
        <Button aria-label="Execute search" onClick={() => handleSearch(query)} disabled={isSearching || isApplying}>
          {isSearching ? <Loader2 className="w-4 h-4 animate-spin" /> : <Search className="w-4 h-4" />}
        </Button>
      </div>

      <div className="grid grid-cols-2 gap-4 max-h-96 overflow-y-auto p-2">
        {results.map((result) => {
          const previewUrl = taskMetadataService.getProviderImagePreviewUrl(provider, result.image_id);

          return (
            <button
              type="button"
              key={result.id}
              className="border rounded-lg overflow-hidden flex flex-col hover:border-primary cursor-pointer transition-colors disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:border-border text-left"
              onClick={() => handleApply(result)}
              disabled={isApplying}
              aria-busy={applyingResultId === result.id}
            >
              {previewUrl ? (
                <img src={previewUrl} alt={result.title} className="w-full h-48 object-cover" />
              ) : (
                <div className="w-full h-48 bg-muted flex items-center justify-center text-muted-foreground text-xs">
                  {t(`${provider}.search.noImage`)}
                </div>
              )}
              <div className="p-2 bg-card">
                <div className="flex items-center justify-between gap-2">
                  <h4 className="font-medium text-sm truncate" title={result.title}>{result.title}</h4>
                  {applyingResultId === result.id && (
                    <Loader2 className="h-4 w-4 animate-spin shrink-0" />
                  )}
                </div>
                <p className="text-xs text-muted-foreground">{result.release_date || t(`${provider}.search.unknownDate`)}</p>
              </div>
            </button>
          );
        })}
        {results.length === 0 && !isSearching && (
          <div className="col-span-2 text-center py-8 text-muted-foreground">
            {t(`${provider}.search.noResults`)}
          </div>
        )}
      </div>

      <div className="flex justify-end pt-2">
        <Button variant="outline" onClick={onCancel}>
          {t("common.cancel", "Cancel")}
        </Button>
      </div>
    </div>
  );
}
