import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { TagBadge } from "@/components/ui/TagBadge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Check, X } from "lucide-react";
import { useState, useEffect } from "react";
import type { Category, CategoryMetadataSource, CategoryReleaseType, CreateCategoryRequest, UpdateCategoryRequest } from "../types/category";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { availableIcons, availableColors, getCategoryIcon } from "../utils/categoryUtils";

interface AddCategoryModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCategoryCreated: (category: CreateCategoryRequest) => Promise<Category | undefined> | void;
  editingCategory?: Category | null;
  onCategoryUpdated?: (categoryId: string, updateData: UpdateCategoryRequest) => Promise<void>;
}

export function AddCategoryModal({ open, onOpenChange, onCategoryCreated, editingCategory, onCategoryUpdated }: AddCategoryModalProps) {
  const { t } = useTranslation();
  const [createForm, setCreateForm] = useState<CreateCategoryRequest>({
    name: "",
    default_tags: [],
    default_directories: [],
    metadata_source: "none",
    release_type: "none",
    color: "#3b82f6",
    icon: "Folder"
  });
  const [tagInput, setTagInput] = useState("");
  const [directoryInput, setDirectoryInput] = useState("");
  

  // Initialize form when modal opens
  useEffect(() => {
    if (open) {
      if (editingCategory) {
        // Editing mode - populate with existing data
        setCreateForm({
          name: editingCategory.name,
          default_tags: [...(editingCategory.default_tags || [])],
          default_directories: [...(editingCategory.default_directories || [])],
          metadata_source: editingCategory.metadata_source || "none",
          release_type: editingCategory.release_type || "none",
          color: editingCategory.color || "#3b82f6",
          icon: editingCategory.icon || "Folder"
        });
      } else {
        // Create mode - reset to defaults
        setCreateForm({
          name: "",
          default_tags: [],
          default_directories: [],
          metadata_source: "none",
          release_type: "none",
          color: "#3b82f6",
          icon: "Folder"
        });
      }
      setTagInput("");
      setDirectoryInput("");
    }
  }, [open, editingCategory]);

  const handleSubmit = async () => {
    if (!createForm.name) {
      toast.error(t('categories.errors.nameRequired'));
      return;
    }

    try {
      if (editingCategory && onCategoryUpdated) {
        // Update existing category
        const updateData: UpdateCategoryRequest = {
          default_tags: createForm.default_tags,
          default_directories: createForm.default_directories,
          metadata_source: createForm.metadata_source,
          release_type: createForm.release_type,
          color: createForm.color,
          icon: createForm.icon
        };
        await onCategoryUpdated(editingCategory.id, updateData);
      } else {
        // Create new category
        await onCategoryCreated(createForm);
      }
      
      // Reset form
      setCreateForm({
        name: "",
        default_tags: [],
        default_directories: [],
        metadata_source: "none",
        release_type: "none",
        color: "#3b82f6",
        icon: "Folder"
      });
      setTagInput("");
      setDirectoryInput("");
      onOpenChange(false);
    } catch (err) {
      const errorMessage = err instanceof Error 
        ? err.message 
        : editingCategory 
          ? t('categories.errors.updateFailed') 
          : t('categories.errors.createFailed');
      toast.error(errorMessage);
    }
  };

  const addTag = () => {
    if (tagInput.trim()) {
      // Split by comma and trim each tag
      const tags = tagInput.split(',').map(tag => tag.trim()).filter(tag => tag.length > 0);
      
      if (tags.length > 0) {
        setCreateForm({
          ...createForm,
          default_tags: [...(createForm.default_tags || []), ...tags]
        });
        setTagInput("");
      }
    }
  };

  const removeTag = (index: number) => {
    setCreateForm({
      ...createForm,
      default_tags: createForm.default_tags?.filter((_, i) => i !== index) || []
    });
  };

  const addDirectory = () => {
    const directory = directoryInput.trim();
    if (!directory || createForm.default_directories?.includes(directory)) {
      return;
    }
    setCreateForm({
      ...createForm,
      default_directories: [...(createForm.default_directories || []), directory]
    });
    setDirectoryInput("");
  };

  const removeDirectory = (index: number) => {
    setCreateForm({
      ...createForm,
      default_directories: createForm.default_directories?.filter((_, i) => i !== index) || []
    });
  };

  const handleMetadataSourceChange = (value: CategoryMetadataSource) => {
    setCreateForm({
      ...createForm,
      metadata_source: value,
    });
  };

  const handleReleaseTypeChange = (value: CategoryReleaseType) => {
    setCreateForm({ ...createForm, release_type: value });
  };

  const handleClose = () => {
    // Reset form when closing
    setCreateForm({
      name: "",
      default_tags: [],
      default_directories: [],
      metadata_source: "none",
      release_type: "none",
      color: "#3b82f6",
      icon: "Folder"
    });
    setTagInput("");
    setDirectoryInput("");
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={(next) => (next ? onOpenChange(next) : handleClose())}>
      <DialogContent className="w-[calc(100%-2rem)] sm:w-full max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex items-center gap-3">
            <div
              className="h-10 w-10 rounded-lg flex items-center justify-center flex-shrink-0"
              style={{ backgroundColor: createForm.color || "#3b82f6" }}
            >
              {(() => {
                const IconComponent = getCategoryIcon(createForm.icon);
                return <IconComponent className="h-6 w-6 text-white" />;
              })()}
            </div>
            <div>
              <DialogTitle className="text-2xl font-bold">
                {editingCategory ? t('categories.edit') : t('categories.createNew')}
              </DialogTitle>
              {editingCategory && (
                <p className="text-sm text-muted-foreground">{editingCategory.name}</p>
              )}
            </div>
          </div>
        </DialogHeader>

        {/* Form Content */}
        <div className="space-y-6">
          {!editingCategory && (
            <div className="space-y-1.5">
              <Label htmlFor="name" className="text-sm">{t('categories.fields.name')} *</Label>
              <Input
                id="name"
                placeholder={t('categories.placeholders.name')}
                value={createForm.name}
                onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                className="h-9"
              />
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="tags" className="text-sm">{t('categories.fields.defaultTags')}</Label>
            <div 
              className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 min-h-[40px] flex-wrap gap-1 items-center"
              onClick={() => document.getElementById('tagInput')?.focus()}
            >
              {createForm.default_tags && createForm.default_tags.map((tag, index) => (
                <TagBadge
                  key={index}
                  tag={tag}
                  size="sm"
                  showIcon={true}
                  showDelete={true}
                  onDelete={() => removeTag(index)}
                />
              ))}
              <input
                id="tagInput"
                type="text"
                placeholder={createForm.default_tags?.length === 0 ? t('categories.placeholders.tags') : ""}
                value={tagInput}
                onChange={(e) => setTagInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    addTag();
                  } else if (e.key === "Backspace" && tagInput === "" && createForm.default_tags && createForm.default_tags.length > 0) {
                    e.preventDefault();
                    removeTag(createForm.default_tags.length - 1);
                  }
                }}
                className="flex-1 min-w-[120px] bg-transparent border-none outline-none text-sm"
              />
            </div>
            <div className="text-xs text-muted-foreground">
              {t('categories.tagsHint')}
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="release-type" className="text-sm">{t("categories.fields.releaseType")}</Label>
              <Select value={createForm.release_type || "none"} onValueChange={(value) => handleReleaseTypeChange(value as CategoryReleaseType)}>
                <SelectTrigger id="release-type" className="h-9"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">{t("categories.releaseType.options.none")}</SelectItem>
                  <SelectItem value="movie">{t("categories.releaseType.options.movie")}</SelectItem>
                  <SelectItem value="series">{t("categories.releaseType.options.series")}</SelectItem>
                  <SelectItem value="os">{t("categories.releaseType.options.os")}</SelectItem>
                  <SelectItem value="game">{t("categories.releaseType.options.game")}</SelectItem>
                  <SelectItem value="book">{t("categories.releaseType.options.book")}</SelectItem>
                  <SelectItem value="music">{t("categories.releaseType.options.music")}</SelectItem>
                  <SelectItem value="software">{t("categories.releaseType.options.software")}</SelectItem>
                  <SelectItem value="audiobook">{t("categories.releaseType.options.audiobook")}</SelectItem>
                  <SelectItem value="comic">{t("categories.releaseType.options.comic")}</SelectItem>
                  <SelectItem value="course">{t("categories.releaseType.options.course")}</SelectItem>
                  <SelectItem value="dataset">{t("categories.releaseType.options.dataset")}</SelectItem>
                  <SelectItem value="rom">{t("categories.releaseType.options.rom")}</SelectItem>
                  <SelectItem value="podcast">{t("categories.releaseType.options.podcast")}</SelectItem>
                  <SelectItem value="anime">{t("categories.releaseType.options.anime")}</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">{t("categories.releaseType.help")}</p>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="metadata-source" className="text-sm">{t('categories.fields.metadataSource')}</Label>
              <Select
                value={createForm.metadata_source || "none"}
                onValueChange={(value) => handleMetadataSourceChange(value as CategoryMetadataSource)}
              >
                <SelectTrigger id="metadata-source" className="h-9">
                  <SelectValue placeholder={t('categories.metadataSource.placeholder')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">{t('categories.metadataSource.options.none')}</SelectItem>
                  <SelectItem value="tgdb">{t('categories.metadataSource.options.tgdb')}</SelectItem>
                  <SelectItem value="tmdb">{t('categories.metadataSource.options.tmdb')}</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">{t('categories.metadataSource.help')}</p>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="directory" className="text-sm">{t('categories.fields.defaultDirectories')}</Label>
            <div className="flex min-h-9 flex-wrap items-center gap-1.5 rounded-md border border-input bg-background px-2 py-1.5">
              {createForm.default_directories?.map((directory, index) => (
                <span key={directory} className="inline-flex max-w-full items-center gap-1 rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
                  <span className="truncate">{directory}</span>
                  <button type="button" onClick={() => removeDirectory(index)} aria-label={t('categories.removeDirectory', { directory })}>
                    <X className="h-3 w-3" />
                  </button>
                </span>
              ))}
              <Input
                id="directory"
                placeholder={t('categories.placeholders.defaultDirectory')}
                value={directoryInput}
                onChange={(event) => setDirectoryInput(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    addDirectory();
                  }
                }}
                className="h-7 min-w-[180px] flex-1 border-0 bg-transparent px-0 shadow-none focus-visible:ring-0"
              />
            </div>
            <p className="text-xs text-muted-foreground">{t('categories.directoriesHint')}</p>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1">
              <Label htmlFor="icon" className="text-sm">{t('categories.fields.icon')}</Label>
              <div className="grid grid-cols-5 gap-0.5">
                {availableIcons.map((iconItem) => {
                  const IconComponent = iconItem.icon;
                  return (
                    <button
                      key={iconItem.name}
                      type="button"
                      className={`w-11 h-11 rounded-md border-2 flex items-center justify-center transition-all hover:scale-105 ${
                        createForm.icon === iconItem.name ? 'border-foreground bg-accent scale-105' : 'border-border hover:bg-accent/50'
                      }`}
                      onClick={() => setCreateForm({ ...createForm, icon: iconItem.name })}
                      title={iconItem.name}
                    >
                      <IconComponent className="h-6 w-6" />
                    </button>
                  );
                })}
              </div>
            </div>

            <div className="space-y-1">
              <Label htmlFor="color" className="text-sm">{t('categories.fields.color')}</Label>
              <div className="grid grid-cols-5 gap-0.5">
                {availableColors.map((color) => (
                  <button
                    key={color.value}
                    type="button"
                    className={`w-11 h-11 rounded-md border-2 transition-transform hover:scale-105 ${
                      createForm.color === color.value ? 'border-foreground scale-105' : 'border-transparent'
                    }`}
                    style={{ backgroundColor: color.value }}
                    onClick={() => setCreateForm({ ...createForm, color: color.value })}
                    title={color.name}
                  />
                ))}
              </div>
            </div>
          </div>
        </div>

        <DialogFooter className="pt-4 border-t">
          <Button
            type="button"
            variant="outline"
            onClick={handleClose}
          >
            {t('common.cancel')}
          </Button>
          <Button onClick={handleSubmit}>
            <Check className="h-4 w-4 mr-2" />
            {editingCategory ? t('common.save') : t('categories.create')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
