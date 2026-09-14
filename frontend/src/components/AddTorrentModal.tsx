import { useEffect, useMemo, useRef, useState } from "react";
import {
  Check,
  ChevronsUpDown,
  Database,
  Download,
  FileText,
  FileUp,
  Folder,
  Globe,
  HardDrive,
  Link,
  Loader2,
  Server,
  X,
} from "lucide-react";
import { useLocation, useNavigate } from "react-router";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Attachment } from "@/components/ui/attachment";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { WorkerIcon } from "@/components/ui/WorkerIcon";
import { SelectCategory } from "@/components/SelectCategory";
import { SelectTags } from "@/components/SelectTags";
import { convertMagnetUriToTaskMagnetLink, torrentService } from "@/services/torrents";
import { workerService } from "@/services/workers";
import { categoryService } from "@/services/categories";
import { taskMetadataService, type ReleaseParseResponse } from "@/services/taskMetadata";
import { useAddTorrent } from "@/contexts/add-torrent-hooks";
import type { AddTorrentMode } from "@/contexts/add-torrent-context";
import { useIsPortraitMobileOrTablet } from "@/hooks/use-portrait-mobile-tablet";
import { formatBytes } from "@/utils/bytes";
import { useTranslation } from "react-i18next";
import type { CreateTaskRequest, TaskMagnetLink } from "@/types/torrent";
import type { Category } from "@/types/category";
import type { Worker } from "@/types/worker";

const finalizeButtonClassName =
  "bg-primary text-primary-foreground shadow-[0_0_30px_rgba(59,130,246,0.5),0_0_60px_rgba(59,130,246,0.22)] transition-shadow hover:shadow-[0_0_38px_rgba(59,130,246,0.65),0_0_72px_rgba(59,130,246,0.3)]";

export function AddTorrentModal() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const isPortraitMobileOrTablet = useIsPortraitMobileOrTablet();
  const { isAddModalOpen, addModalMode, closeAddModal, addPendingTorrent, removePendingTorrent } = useAddTorrent();

  const [workers, setWorkers] = useState<Worker[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [mode, setMode] = useState<AddTorrentMode>("magnet");
  const [isCreating, setIsCreating] = useState(false);
  const [createError, setCreateError] = useState("");
  const [selectedWorkerId, setSelectedWorkerId] = useState<string>("");
  const [magnetUri, setMagnetUri] = useState("");
  const [torrentFile, setTorrentFile] = useState<File | null>(null);
  const [selectedCategoryId, setSelectedCategoryId] = useState<string>("");
  const [category, setCategory] = useState("");
  const [directory, setDirectory] = useState("");
  const [directoryDropdownOpen, setDirectoryDropdownOpen] = useState(false);
  const [tags, setTags] = useState<string[]>([]);
  const [displayName, setDisplayName] = useState("");
  const [releaseSuggestion, setReleaseSuggestion] = useState<ReleaseParseResponse | null>(null);
  const [userEdited, setUserEdited] = useState({ category: false, directory: false, tags: false, displayName: false });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [workerDropdownOpen, setWorkerDropdownOpen] = useState(false);
  const [parsedMagnetLink, setParsedMagnetLink] = useState<TaskMagnetLink | null>(null);
  const workerDropdownRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const isFileMode = mode === "file";

  // Read via refs (not as effect deps) inside applySuggestion below: editing
  // an unrelated field (tags, directory, category) must not re-trigger the
  // debounced release-parse network call — in file mode that would re-upload
  // the whole .torrent file — for a rawName/torrentFile that hasn't changed.
  const categoriesRef = useRef(categories);
  const selectedCategoryIdRef = useRef(selectedCategoryId);
  const userEditedRef = useRef(userEdited);
  const releaseSuggestionRef = useRef(releaseSuggestion);
  useEffect(() => {
    categoriesRef.current = categories;
  }, [categories]);
  useEffect(() => {
    selectedCategoryIdRef.current = selectedCategoryId;
  }, [selectedCategoryId]);
  useEffect(() => {
    userEditedRef.current = userEdited;
  }, [userEdited]);
  useEffect(() => {
    releaseSuggestionRef.current = releaseSuggestion;
  }, [releaseSuggestion]);

  const activeWorkers = useMemo(() => workers.filter((worker) => worker.status === "ACTIVE"), [workers]);
  const selectedWorker = useMemo(
    () => activeWorkers.find((worker) => worker.uuid === selectedWorkerId),
    [activeWorkers, selectedWorkerId]
  );
  const freeSpace = useMemo(() => selectedWorker?.instance?.server?.free_space_on_disk || 0, [selectedWorker]);
  const selectedCategory = useMemo(
    () => categories.find((item) => item.id === selectedCategoryId),
    [categories, selectedCategoryId]
  );
  const categoryDirectories = selectedCategory?.default_directories || [];

  useEffect(() => {
    if (!isAddModalOpen) {
      return;
    }

    setSelectedWorkerId("");
    setMode(addModalMode);
    setSelectedCategoryId("");
    setMagnetUri("");
    setTorrentFile(null);
    setCategory("");
    setDirectory("");
    setTags([]);
    setDisplayName("");
    setReleaseSuggestion(null);
    setUserEdited({ category: false, directory: false, tags: false, displayName: false });
    setErrors({});
    setParsedMagnetLink(null);
    setCreateError("");
    setIsCreating(false);
    setWorkerDropdownOpen(false);
    setDirectoryDropdownOpen(false);

    let cancelled = false;
    workerService
      .listWorkers()
      .then((response) => {
        if (!cancelled && response.data) {
          setWorkers(response.data);
        }
      })
      .catch((error) => {
        console.error("Failed to load workers:", error);
      });

    categoryService.listCategories().then((response) => {
      if (!cancelled && response.data) {
        setCategories(response.data);
        // Categories can still be loading when a magnet/file finishes
        // parsing (e.g. the user pastes a magnet immediately on open), so
        // that first applySuggestion() call ran against an empty category
        // list and couldn't match one. Update the ref in-place (the effect
        // that normally syncs it hasn't run yet) and reapply the stored
        // high-confidence suggestion now that categories are here.
        categoriesRef.current = response.data;
        if (releaseSuggestionRef.current) {
          applySuggestion(releaseSuggestionRef.current);
        }
      }
    }).catch((error) => console.error("Failed to load categories:", error));

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- applySuggestion is intentionally excluded: it's recreated every render, and this effect must only run when the modal opens/changes mode, not on every render
  }, [isAddModalOpen, addModalMode]);

  useEffect(() => {
    if (!isAddModalOpen || selectedWorkerId !== "") {
      return;
    }

    setSelectedWorkerId(activeWorkers.length > 0 ? activeWorkers[0].uuid : "");
  }, [isAddModalOpen, activeWorkers, selectedWorkerId]);

  useEffect(() => {
    if (mode === "magnet" && magnetUri.trim() && magnetUri.startsWith("magnet:")) {
      setParsedMagnetLink(convertMagnetUriToTaskMagnetLink(magnetUri.trim()));
      return;
    }

    setParsedMagnetLink(null);
  }, [magnetUri, mode]);

  const mergeTags = (base: string[], suggested: string[]): string[] => {
    const byScope = new Map<string, string>();
    const plain: string[] = [];
    for (const tag of [...base, ...suggested]) {
      const separator = tag.indexOf("::");
      if (separator > 0) {
        byScope.set(tag.slice(0, separator).toLowerCase(), tag);
      } else if (!plain.includes(tag)) {
        plain.push(tag);
      }
    }
    return [...plain, ...byScope.values()];
  };

  // When multiple categories share a release_type, prefer the one whose
  // default_tags overlap the most with the parsed release's suggested tags
  // (e.g. an "Anime" category with tag "type::anime" beats a generic "Series"
  // one for the same release_type), falling back to the first match.
  const pickBestCategoryMatch = (candidates: Category[], suggestedTags: string[]): Category => {
    const suggestedSet = new Set(suggestedTags.map((tag) => tag.toLowerCase()));
    return candidates.reduce((best, candidate) => {
      const candidateScore = (candidate.default_tags || []).filter((tag) => suggestedSet.has(tag.toLowerCase())).length;
      const bestScore = (best.default_tags || []).filter((tag) => suggestedSet.has(tag.toLowerCase())).length;
      return candidateScore > bestScore ? candidate : best;
    }, candidates[0]);
  };

  // Resets any fields that were auto-filled from a previous parse suggestion
  // and haven't been hand-edited by the user, so switching to a different
  // magnet/file (or one that no longer parses with high confidence) doesn't
  // leave stale values attributed to the wrong release.
  const clearAutoFilledFields = () => {
    const userEdited = userEditedRef.current;
    setReleaseSuggestion(null);
    if (!userEdited.displayName) {
      setDisplayName("");
    }
    if (!userEdited.category) {
      setSelectedCategoryId("");
      setCategory("");
    }
    if (!userEdited.tags) {
      setTags([]);
    }
    if (!userEdited.directory) {
      setDirectory("");
    }
  };

  const applySuggestion = (suggestion: ReleaseParseResponse) => {
    if (suggestion.release.confidence !== "high") {
      clearAutoFilledFields();
      return;
    }
    const categories = categoriesRef.current;
    const selectedCategoryId = selectedCategoryIdRef.current;
    const userEdited = userEditedRef.current;
    setReleaseSuggestion(suggestion);
    if (!userEdited.displayName) {
      setDisplayName(suggestion.display_name);
    }
    const selectedCategory = categories.find((item) => item.id === selectedCategoryId);
    if (!userEdited.tags) {
      setTags(mergeTags(selectedCategory?.default_tags || [], suggestion.tags));
    }
    if (!userEdited.category && suggestion.release.type !== "unknown") {
      const candidates = categories.filter((item) => item.release_type === suggestion.release.type);
      const categoryMatch = candidates.length > 0 ? pickBestCategoryMatch(candidates, suggestion.tags) : undefined;
      if (categoryMatch) {
        setSelectedCategoryId(categoryMatch.id);
        setCategory(categoryMatch.name);
        if (!userEdited.tags) setTags(mergeTags(categoryMatch.default_tags || [], suggestion.tags));
        if (!userEdited.directory) {
          setDirectory(categoryMatch.default_directories?.[0] || "");
        }
      }
    }
  };

  useEffect(() => {
    if (mode !== "magnet") {
      return;
    }
    const rawName = parsedMagnetLink?.display_name.trim();
    if (!rawName) {
      clearAutoFilledFields();
      return;
    }
    let cancelled = false;
    const timeout = window.setTimeout(() => {
      taskMetadataService.parseRelease(rawName).then((response) => {
        if (!cancelled && response.data) {
          applySuggestion(response.data);
        }
      }).catch((error) => console.error("Failed to parse release name:", error));
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- categories/selectedCategoryId/userEdited are read via refs so editing an unrelated field doesn't re-trigger this parse
  }, [mode, parsedMagnetLink?.display_name]);

  useEffect(() => {
    if (mode !== "file") {
      return;
    }
    if (!torrentFile) {
      clearAutoFilledFields();
      return;
    }
    let cancelled = false;
    taskMetadataService.parseReleaseFile(torrentFile).then((response) => {
      if (!cancelled && response.data) {
        applySuggestion(response.data);
      }
    }).catch((error) => console.error("Failed to parse torrent file:", error));
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- categories/selectedCategoryId/userEdited are read via refs so editing an unrelated field doesn't re-upload the file
  }, [mode, torrentFile]);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (workerDropdownRef.current && !workerDropdownRef.current.contains(event.target as Node)) {
        setWorkerDropdownOpen(false);
      }
    };

    if (!workerDropdownOpen) {
      return;
    }

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [workerDropdownOpen]);

  const isBusy = isCreating;

  const validate = (): boolean => {
    const nextErrors: Record<string, string> = {};

    if (isFileMode) {
      if (!torrentFile) {
        nextErrors.magnetUri = t("torrents.addModal.errors.fileRequired");
      } else if (!torrentFile.name.toLowerCase().endsWith(".torrent")) {
        nextErrors.magnetUri = t("torrents.addModal.errors.fileInvalidExtension");
      }
    } else if (!magnetUri.trim()) {
      nextErrors.magnetUri = t("torrents.addModal.errors.magnetRequired");
    } else if (!magnetUri.startsWith("magnet:")) {
      nextErrors.magnetUri = t("torrents.addModal.errors.magnetInvalidPrefix");
    } else if (!parsedMagnetLink?.hash) {
      nextErrors.magnetUri = t("torrents.addModal.errors.magnetInvalidPrefix");
    }

    if (!selectedWorkerId) {
      nextErrors.worker = t("torrents.addModal.errors.workerRequired");
    }

    setErrors({
      magnetUri: nextErrors.magnetUri || "",
      category: nextErrors.category || "",
      worker: nextErrors.worker || "",
      tags: nextErrors.tags || "",
    });

    return Object.keys(nextErrors).length === 0;
  };

  const handleCategoryChange = (categoryId: string, nextCategory?: Category) => {
    setUserEdited((current) => ({ ...current, category: true }));
    setSelectedCategoryId(categoryId);
    setErrors((current) => ({ ...current, category: "" }));

    if (categoryId && nextCategory) {
      setCategories((current) => {
        const existingIndex = current.findIndex((item) => item.id === nextCategory.id);
        if (existingIndex < 0) {
          return [...current, nextCategory];
        }
        return current.map((item) => item.id === nextCategory.id ? nextCategory : item);
      });
      setCategory(nextCategory.name);
      setTags(mergeTags([...(nextCategory.default_tags || [])], releaseSuggestion?.tags || []));
      setDirectory(nextCategory.default_directories?.[0] || "");
      setDirectoryDropdownOpen(false);
      return;
    }

    setCategory("");
    setTags([]);
    setDirectory("");
    setDirectoryDropdownOpen(false);
  };

  const handleWorkerChange = (workerId: string) => {
    setSelectedWorkerId(workerId);
    setErrors((current) => ({ ...current, worker: "" }));
    setWorkerDropdownOpen(false);
    setDirectoryDropdownOpen(false);
  };

  const handleModeChange = (nextMode: string) => {
    const next = nextMode as AddTorrentMode;
    if (next === mode) {
      return;
    }

    setMode(next);
    setMagnetUri("");
    setTorrentFile(null);
    setParsedMagnetLink(null);
    setSelectedCategoryId("");
    setCategory("");
    setDirectory("");
    setTags([]);
    setDisplayName("");
    setReleaseSuggestion(null);
    setUserEdited({ category: false, directory: false, tags: false, displayName: false });
    setErrors({});
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  // Applies the user-edited display name after a torrent is created
  // (was_created=false means it was a dedupe match against an existing
  // task, which must keep its own name). One retry covers a fresh task's
  // metadata record not being registered yet; if it still fails the
  // warning stays visible until dismissed since the torrent itself did
  // get added and nothing else will retry the rename for the user.
  const applyDisplayName = async (hash: string, wasCreated: boolean) => {
    if (!wasCreated || !displayName.trim()) {
      return;
    }
    let nameResponse = await taskMetadataService.updateName(hash, displayName.trim());
    if (nameResponse.error) {
      await new Promise((resolve) => setTimeout(resolve, 500));
      nameResponse = await taskMetadataService.updateName(hash, displayName.trim());
    }
    if (nameResponse.error) {
      toast.warning(t("torrents.notifications.addSuccessDisplayNameFailed"), { duration: Infinity });
    }
  };

  const handleSubmitFile = async () => {
    if (!torrentFile) {
      return;
    }

    const workerId = selectedWorkerId;
    setIsCreating(true);
    setCreateError("");
    closeAddModal();
    if (location.pathname !== "/torrents") {
      navigate("/torrents");
    }

    try {
      const response = await torrentService.createTaskFromFile(workerId, torrentFile, {
        category: category.trim(),
        directory: directory.trim() || undefined,
        tags,
      });

      if (response.error) {
        throw new Error(response.error);
      }

      if (response.data?.hash) {
        await applyDisplayName(response.data.hash, !!response.data.was_created);
      }

      // Sem hash conhecido de antemão: o card real chega via evento WS
      // TORRENT_ADDED (sem placeholder otimista, diferente do fluxo magnet).
      toast.success(t("torrents.notifications.addSuccess"));
    } catch (error) {
      const errorMessage = error instanceof Error && error.message ? error.message : t("torrents.error");
      toast.error(t("torrents.notifications.addError", { error: errorMessage }));
    } finally {
      setIsCreating(false);
    }
  };

  const handleSubmitMagnet = async () => {
    if (!parsedMagnetLink?.hash) {
      return;
    }

    const hash = parsedMagnetLink.hash.toLowerCase();
    const parsedSize = parsedMagnetLink.exact_length
      ? Number.parseInt(parsedMagnetLink.exact_length, 10)
      : undefined;

    const taskData: CreateTaskRequest = {
      magnet_uri: magnetUri.trim(),
      category: category.trim(),
      tags,
      ...(directory.trim() && { directory: directory.trim() }),
    };
    const workerId = selectedWorkerId;

    setIsCreating(true);
    setCreateError("");

    addPendingTorrent({
      hash,
      name: parsedMagnetLink.display_name || hash,
      size: Number.isFinite(parsedSize) ? parsedSize : undefined,
      workerId,
      category: taskData.category,
      addedAt: Date.now(),
    });

    closeAddModal();
    if (location.pathname !== "/torrents") {
      navigate("/torrents");
    }

    try {
      const response = await torrentService.createTask(workerId, taskData);

      if (response.error || !response.data?.hash) {
        throw new Error(response.error || t("torrents.addModal.errors.taskHashMissing"));
      }

      await applyDisplayName(response.data.hash, !!response.data.was_created);

      toast.success(t("torrents.notifications.addSuccess"));
    } catch (error) {
      removePendingTorrent(hash);
      const errorMessage = error instanceof Error && error.message ? error.message : t("torrents.error");
      toast.error(t("torrents.notifications.addError", { error: errorMessage }));
    } finally {
      setIsCreating(false);
    }
  };

  const handleSubmit = async () => {
    if (isBusy || !validate()) {
      return;
    }

    if (isFileMode) {
      await handleSubmitFile();
    } else {
      await handleSubmitMagnet();
    }
  };

  const handleRequestClose = () => {
    if (!isBusy) {
      closeAddModal();
    }
  };

  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && isAddModalOpen && !isBusy) {
        closeAddModal();
      }
    };

    if (!isAddModalOpen) {
      return;
    }

    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, [isAddModalOpen, isBusy, closeAddModal]);

  if (!isAddModalOpen) {
    return null;
  }

  const content = (
    <form
      className="flex min-h-0 min-w-0 flex-1 flex-col overflow-x-hidden"
      onSubmit={(event) => {
        event.preventDefault();
        void handleSubmit();
      }}
    >
      <div className="z-20 flex items-center justify-between border-b bg-card p-4 sm:p-6">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10">
            <Download className="h-4 w-4 text-primary" />
          </div>
          <div>
            <h2 className="text-2xl font-bold">{t("torrents.addTorrent")}</h2>
            <p className="text-sm text-muted-foreground">{t("torrents.addModal.subtitle")}</p>
          </div>
        </div>
        <Button variant="ghost" size="icon" onClick={handleRequestClose} className="h-8 w-8" type="button">
          <X className="h-4 w-4" />
        </Button>
      </div>

      <ScrollArea className="h-full min-h-0 min-w-0">
        <div className="space-y-6 p-4 pt-5 sm:p-6">
          {createError && (
            <p
              role="alert"
              className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {createError}
            </p>
          )}

          <Tabs value={mode} onValueChange={handleModeChange}>
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="magnet" className="gap-2">
                <Link className="h-4 w-4" />
                {t("torrents.addModal.source.magnet")}
              </TabsTrigger>
              <TabsTrigger value="file" className="gap-2">
                <FileUp className="h-4 w-4" />
                {t("torrents.addModal.source.upload")}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="file" className="mt-5 space-y-2">
              <Label htmlFor="torrentFile" className="flex items-center gap-2">
                <FileUp className="h-4 w-4" />
                {t("torrents.addModal.file.label")} <span className="text-destructive">*</span>
              </Label>
              <input
                ref={fileInputRef}
                id="torrentFile"
                type="file"
                accept=".torrent"
                className="hidden"
                disabled={isBusy}
                onChange={(event) => {
                  setTorrentFile(event.target.files?.[0] ?? null);
                  setErrors((current) => ({ ...current, magnetUri: "" }));
                }}
              />
              <Attachment
                disabled={isBusy}
                aria-invalid={!!errors.magnetUri}
                selected={!!torrentFile}
                onClick={() => fileInputRef.current?.click()}
                onRemove={() => {
                  setTorrentFile(null);
                  setReleaseSuggestion(null);
                  if (fileInputRef.current) {
                    fileInputRef.current.value = "";
                  }
                }}
                removeLabel={t("torrents.addModal.file.remove")}
                className={errors.magnetUri ? "border-destructive" : ""}
                icon={torrentFile ? <FileText className="h-7 w-7" /> : <FileUp className="h-5 w-5" />}
                description={torrentFile ? t("torrents.addModal.file.details", { size: formatBytes(torrentFile.size) }) : t("torrents.addModal.file.help")}
              >
                {torrentFile ? torrentFile.name : t("torrents.addModal.file.placeholder")}
              </Attachment>
              {errors.magnetUri && <p className="text-sm text-destructive">{errors.magnetUri}</p>}
            </TabsContent>

            <TabsContent value="magnet" className="mt-5 space-y-2">
              <Label htmlFor="magnetUri" className="flex items-center gap-2">
                <Link className="h-4 w-4" />
                {t("torrents.addModal.magnetUri.label")} <span className="text-destructive">*</span>
              </Label>
              <Textarea
                id="magnetUri"
                placeholder={t("torrents.addModal.magnetUri.placeholder")}
                value={magnetUri}
                onChange={(event) => {
                  setMagnetUri(event.target.value);
                  setErrors((current) => ({ ...current, magnetUri: "" }));
                }}
                onKeyDown={(event) => {
                  if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
                    event.preventDefault();
                    void handleSubmit();
                  }
                }}
                className={`min-h-28 resize-y whitespace-pre-wrap break-all font-mono text-sm ${errors.magnetUri ? "border-destructive" : ""}`}
                disabled={isBusy}
              />
              {errors.magnetUri && <p className="text-sm text-destructive">{errors.magnetUri}</p>}

              {parsedMagnetLink && (
                <div className="mt-3 min-w-0 space-y-3 overflow-hidden rounded-lg border bg-muted/50 p-4">
                  <div className="mb-2 flex items-center justify-between gap-2">
                    <div className="flex min-w-0 items-center gap-2">
                      <Database className="h-4 w-4 shrink-0 text-primary" />
                      <span className="text-sm font-medium text-foreground">{t("torrents.addModal.magnetInfo.title")}</span>
                    </div>
                    {parsedMagnetLink.trackers.length > 0 && (
                      <span
                        aria-label={t("torrents.addModal.magnetInfo.trackersCount", { count: parsedMagnetLink.trackers.length })}
                        className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-muted-foreground"
                        title={t("torrents.addModal.magnetInfo.trackers")}
                      >
                        <Globe className="h-3.5 w-3.5" aria-hidden="true" />
                        {parsedMagnetLink.trackers.length}
                      </span>
                    )}
                  </div>

                {parsedMagnetLink.display_name && (
                  <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-start gap-2 text-xs">
                    <FileText className="h-3 w-3 text-muted-foreground" />
                    <div className="min-w-0">
                      <span className="block text-muted-foreground">{t("torrents.addModal.magnetInfo.name")}:</span>
                      <span className="block break-words font-medium text-foreground">{parsedMagnetLink.display_name}</span>
                    </div>
                  </div>
                )}

                {parsedMagnetLink.exact_length && (
                  <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-start gap-2 text-xs">
                    <HardDrive className="h-3 w-3 text-muted-foreground" />
                    <div className="min-w-0">
                      <span className="block text-muted-foreground">{t("torrents.addModal.magnetInfo.size")}:</span>
                      <span className="block break-words font-medium text-foreground">
                        {formatBytes(Number.parseInt(parsedMagnetLink.exact_length, 10))}
                      </span>
                    </div>
                  </div>
                )}
                </div>
              )}
              {releaseSuggestion?.release.confidence === "high" && (
                <p className="text-xs text-muted-foreground">{t("torrents.addModal.release.review")}</p>
              )}
            </TabsContent>
          </Tabs>

          <div className="space-y-2">
            <Label htmlFor="displayName" className="flex items-center gap-2">
              <FileText className="h-4 w-4" /> {t("torrents.addModal.release.displayName")}
              {releaseSuggestion?.release.confidence === "high" && <span className="text-xs text-blue-600">({t("torrents.addModal.release.suggested")})</span>}
            </Label>
            <Input
              id="displayName"
              value={displayName}
              placeholder={t("torrents.addModal.release.displayNamePlaceholder")}
              onChange={(event) => {
                setUserEdited((current) => ({ ...current, displayName: true }));
                setDisplayName(event.target.value);
              }}
              disabled={isBusy}
            />
          </div>

          <SelectCategory
            selectedCategoryId={selectedCategoryId}
            onCategoryChange={handleCategoryChange}
            label={t("torrents.addModal.category.label")}
            required={false}
            error={errors.category}
            showAddButton={true}
          />

          <div className="space-y-2">
            <Label htmlFor="directory" className="flex items-center gap-2">
              <Folder className="h-4 w-4" />
              {t("torrents.addModal.directory.label")}
              <span className="text-xs text-muted-foreground">({t("torrents.addModal.directory.optional")})</span>
              {selectedCategoryId && directory && (
                <span className="ml-2 text-xs text-blue-600">({t("torrents.addModal.directory.autoFilled")})</span>
              )}
            </Label>
            {categoryDirectories.length > 0 ? (
              <Popover open={directoryDropdownOpen} onOpenChange={setDirectoryDropdownOpen}>
                <PopoverTrigger asChild>
                  <Button
                    id="directory"
                    type="button"
                    variant="outline"
                    role="combobox"
                    aria-expanded={directoryDropdownOpen}
                    disabled={isBusy}
                    className="w-full justify-between font-normal"
                  >
                    <span className="truncate">{directory || t("torrents.addModal.directory.select")}</span>
                    <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
                  <Command>
                    <CommandInput placeholder={t("torrents.addModal.directory.searchPlaceholder")} />
                    <CommandList>
                      <CommandEmpty>{t("torrents.addModal.directory.noResults")}</CommandEmpty>
                      <CommandGroup>
                        {categoryDirectories.map((categoryDirectory) => (
                          <CommandItem
                            key={categoryDirectory}
                            value={categoryDirectory}
                            onSelect={() => {
                              setUserEdited((current) => ({ ...current, directory: true }));
                              setDirectory(categoryDirectory);
                              setDirectoryDropdownOpen(false);
                            }}
                          >
                            <Check className={`mr-2 h-4 w-4 ${directory === categoryDirectory ? "opacity-100" : "opacity-0"}`} />
                            <span className="truncate font-mono">{categoryDirectory}</span>
                          </CommandItem>
                        ))}
                      </CommandGroup>
                    </CommandList>
                  </Command>
                </PopoverContent>
              </Popover>
            ) : (
              <Input
                id="directory"
                type="text"
                placeholder={t("torrents.addModal.directory.placeholder")}
                value={directory}
                onChange={(event) => {
                  setUserEdited((current) => ({ ...current, directory: true }));
                  setDirectory(event.target.value);
                }}
                disabled={isBusy}
              />
            )}
          </div>

          <SelectTags
            tags={tags}
            onTagsChange={(nextTags) => {
              setUserEdited((current) => ({ ...current, tags: true }));
              setTags(nextTags);
              if (nextTags.length > 0) {
                setErrors((current) => ({ ...current, tags: "" }));
              }
            }}
            label={t("torrents.addModal.tags.label")}
            required={false}
            error={errors.tags}
            placeholder={t("torrents.addModal.tags.placeholder")}
            labelAccessory={selectedCategoryId && tags.length > 0 ? (
              <span className="text-xs font-normal text-blue-600">({t("torrents.addModal.tags.autoFilled")})</span>
            ) : undefined}
            disabled={isBusy}
          />

          <div className="space-y-2">
            <Label htmlFor="worker" className="flex items-center gap-2">
              <Server className="h-4 w-4" />
              {t("torrents.addModal.worker.label")} <span className="text-destructive">*</span>
              {selectedWorkerId && freeSpace > 0 && (
                <span className="ml-1 inline-flex items-center gap-1 text-xs font-normal text-muted-foreground">
                  <HardDrive className="h-3 w-3" />
                  {t("torrents.addModal.worker.freeSpace")} {formatBytes(freeSpace)}
                </span>
              )}
            </Label>
            <div className="relative" ref={workerDropdownRef}>
              <Button
                type="button"
                variant="outline"
                onClick={() => setWorkerDropdownOpen(!workerDropdownOpen)}
                disabled={activeWorkers.length === 0 || isBusy}
                className={`w-full justify-between ${errors.worker ? "border-destructive" : ""}`}
              >
                <div className="flex items-center gap-2">
                  {selectedWorker ? (
                    <WorkerIcon iconName={selectedWorker.icon} color={selectedWorker.color} size="sm" />
                  ) : (
                    <Server className="h-4 w-4 text-muted-foreground" />
                  )}
                  <span className="truncate">
                    {selectedWorker?.name ||
                      (activeWorkers.length === 0
                        ? t("torrents.addModal.worker.noneAvailable")
                        : t("torrents.addModal.worker.select"))}
                  </span>
                </div>
                <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
              </Button>

              {workerDropdownOpen && activeWorkers.length > 0 && (
                <div className="absolute bottom-full z-50 mb-1 max-h-60 w-full overflow-auto rounded-md border border-border bg-background shadow-lg">
                  {activeWorkers.map((worker) => (
                    <button
                      key={worker.uuid}
                      type="button"
                      onClick={() => handleWorkerChange(worker.uuid)}
                      className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent hover:text-accent-foreground"
                    >
                      <WorkerIcon iconName={worker.icon} color={worker.color} size="md" />
                      <span className="flex-1 truncate">{worker.name}</span>
                      {selectedWorkerId === worker.uuid && <Check className="h-4 w-4 text-primary" />}
                    </button>
                  ))}
                </div>
              )}
            </div>

            {errors.worker && <p className="text-sm text-destructive">{errors.worker}</p>}
          </div>
        </div>
      </ScrollArea>

      <div className="z-20 border-t bg-card px-4 py-4 sm:px-6">
        <div className="flex items-center justify-between gap-3">
          <Button type="button" variant="outline" onClick={handleRequestClose} disabled={isBusy}>
            {t("common.cancel")}
          </Button>

          <Button
            type="submit"
            disabled={isBusy || activeWorkers.length === 0}
            className={`min-w-28 ${finalizeButtonClassName}`}
          >
            {isCreating ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                {t("torrents.addModal.actions.creating")}
              </>
            ) : (
              t("torrents.addModal.actions.add")
            )}
          </Button>
        </div>
      </div>
    </form>
  );

  if (isPortraitMobileOrTablet) {
    return (
      <Sheet
        open={isAddModalOpen}
        onOpenChange={(open) => {
          if (!open) {
            handleRequestClose();
          }
        }}
      >
        <SheetContent
          side="bottom"
          hideClose
          className="flex h-[calc(100dvh-0.5rem)] w-full max-w-none flex-col gap-0 overflow-x-hidden rounded-t-2xl border-x border-t p-0"
        >
          {content}
        </SheetContent>
      </Sheet>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <button
        type="button"
        aria-label={t("common.cancel")}
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={handleRequestClose}
      />
      <div className="relative mx-2 grid max-h-[calc(100dvh-1rem)] w-full max-w-2xl grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden rounded-lg border bg-card shadow-lg sm:mx-4">
        {content}
      </div>
    </div>
  );
}
