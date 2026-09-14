import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { 
  Folder, 
  Plus, 
  Trash2, 
  Search, 
  Loader2, 
  RefreshCw,
  FolderOpen,
  Pencil,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { categoryService } from "./services/categories";
import type { Category, CreateCategoryRequest, UpdateCategoryRequest } from "./types/category";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { AddCategoryModal } from "./components/AddCategoryModal";
import { TagBadge } from "./components/ui/TagBadge";
import { getCategoryIcon } from "./utils/categoryUtils";

function Categories() {
  const { t } = useTranslation();
  const [categories, setCategories] = useState<Category[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState("");
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingCategory, setEditingCategory] = useState<Category | null>(null);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [categoryToDelete, setCategoryToDelete] = useState<Category | null>(null);
  

  const loadCategories = useCallback(async () => {
    try {
      setLoading(true);
      const response = await categoryService.listCategories();
      
      if (response.error) {
        toast.error(response.error);
      } else if (response.data) {
        setCategories(response.data);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('categories.errors.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  // Load categories on component mount
  useEffect(() => {
    loadCategories();
  }, [loadCategories]);


  const handleDeleteCategory = async () => {
    if (!categoryToDelete) return;
    
    try {
      const response = await categoryService.deleteCategory(categoryToDelete.id);
      if (response.error) {
        toast.error(response.error);
      } else {
        setCategories(categories.filter(cat => cat.id !== categoryToDelete.id));
        toast.success(t('categories.notifications.deleteSuccess'));
        setShowDeleteModal(false);
        setCategoryToDelete(null);
        setEditingCategory(null);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('categories.errors.deleteFailed'));
    }
  };

  const handleCreateCategory = async (createForm: CreateCategoryRequest) => {
    try {
      const response = await categoryService.createCategory(createForm);
      if (response.error) {
        toast.error(response.error);
        throw new Error(response.error);
      } else if (response.data) {
        setCategories([...categories, response.data]);
        toast.success(t('categories.notifications.createSuccess'));
        return response.data;
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('categories.errors.createFailed'));
      throw err;
    }
  };

  const handleUpdateCategory = async (categoryId: string, updateData: UpdateCategoryRequest): Promise<void> => {
    try {
      const response = await categoryService.updateCategory(categoryId, updateData);
      if (response.error) {
        toast.error(response.error);
        throw new Error(response.error);
      } else if (response.data) {
        // Update the categories list
        const updatedCategories = categories.map(cat => cat.id === categoryId && response.data ? response.data : cat);
        setCategories(updatedCategories);
        
        // Update the editing category if it's the same one being updated
        if (editingCategory && editingCategory.id === categoryId) {
          setEditingCategory(response.data);
        }
        
        toast.success(t('categories.notifications.updateSuccess'));
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('categories.errors.updateFailed'));
      throw err;
    }
  };

  const startEditCategory = (category: Category) => {
    setEditingCategory(category);
    setShowCreateModal(true);
  };

  const handleModalClose = (open: boolean) => {
    setShowCreateModal(open);
    if (!open) {
      setEditingCategory(null);
    }
  };




  // Filter categories
  const filteredCategories = categories
    .filter(cat => cat.name.toLowerCase().includes(searchTerm.toLowerCase()));

  return (
    <div className="space-y-6">
      
      {/* Header */}
      <div className="flex flex-col sm:flex-row gap-4 justify-between items-start sm:items-center">
        <div className="flex items-center gap-3">
          <div className="h-12 w-12 rounded-xl bg-primary/10 flex items-center justify-center">
            <FolderOpen className="h-6 w-6 text-primary" />
          </div>
          <div>
            <h1 className="text-2xl font-bold">{t('categories.title')}</h1>
            <p className="text-sm text-muted-foreground">{t('categories.subtitle')}</p>
          </div>
        </div>
        <div className="flex gap-2 w-full sm:w-auto justify-between sm:justify-end">
          <Button onClick={loadCategories} variant="outline" size="icon" aria-label="Refresh categories">
            <RefreshCw className="h-4 w-4" />
          </Button>
          <Button onClick={() => setShowCreateModal(true)} size="sm">
            <Plus className="h-4 w-4 mr-2" />
            {t('categories.addCategory')}
          </Button>
        </div>
      </div>


      {/* Search */}
      <div className="flex flex-col sm:flex-row gap-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={t('categories.search')}
            className="pl-10"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>
      </div>

      {/* Categories List */}
      {loading ? (
        <Card>
          <CardContent className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            <span className="ml-2 text-muted-foreground">{t('categories.loading')}</span>
          </CardContent>
        </Card>
      ) : filteredCategories.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Folder className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">{t('categories.noCategories')}</h3>
            <p className="text-muted-foreground text-center mb-4">{t('categories.noCategoriesDesc')}</p>
            <Button onClick={() => setShowCreateModal(true)}>
              <Plus className="h-4 w-4 mr-2" />
              {t('categories.addCategory')}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <>
          {/* Desktop Table View */}
          <div className="hidden md:block">
            <Card className="py-0 overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b">
                      <th className="text-left p-3 font-medium text-muted-foreground text-sm">{t('categories.fields.name')}</th>
                      <th className="text-left p-3 font-medium text-muted-foreground text-sm">{t('categories.fields.defaultDirectory')}</th>
                      <th className="text-left p-3 font-medium text-muted-foreground text-sm">{t('categories.fields.defaultTags')}</th>
                      <th className="text-left p-3 font-medium text-muted-foreground text-sm">{t('categories.fields.metadataSource')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredCategories.map((category) => (
                      <tr
                        key={category.id}
                        className="hover:bg-accent/50 transition-colors border-b last:border-b-0"
                      >
                        <td className="p-3">
                          <div className="flex items-center gap-3">
                            <div
                              className="w-9 h-9 rounded-md flex items-center justify-center flex-shrink-0"
                              style={{ backgroundColor: category.color || "#3b82f6" }}
                            >
                              {(() => {
                                const IconComponent = getCategoryIcon(category.icon);
                                return <IconComponent className="h-4.5 w-4.5 text-white" />;
                              })()}
                            </div>
                            <h3 className="font-medium text-sm truncate">{category.name}</h3>
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon"
                              className="h-8 w-8 shrink-0"
                              onClick={() => startEditCategory(category)}
                              aria-label={t("categories.editCategory", { defaultValue: "Edit {{name}}", name: category.name })}
                            >
                              <Pencil className="h-4 w-4" />
                            </Button>
                          </div>
                        </td>
                        <td className="p-3">
                          {category.default_directories?.length > 0 && (
                            <div className="flex min-w-0 flex-wrap items-center gap-1">
                              <Folder className="h-3 w-3 text-muted-foreground flex-shrink-0" />
                              {category.default_directories.map((directory) => (
                                <span key={directory} className="max-w-[240px] truncate rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
                                  {directory}
                                </span>
                              ))}
                            </div>
                          )}
                        </td>
                        <td className="p-3">
                          {category.default_tags && category.default_tags.length > 0 && (
                            <div className="flex gap-1 flex-wrap items-center">
                              {category.default_tags.slice(0, 3).map((tag) => (
                                <TagBadge
                                  key={tag}
                                  tag={tag}
                                  size="sm"
                                  showIcon={false}
                                />
                              ))}
                              {category.default_tags.length > 3 && (
                                <span className="text-xs text-muted-foreground">
                                  +{category.default_tags.length - 3}
                                </span>
                              )}
                            </div>
                          )}
                        </td>
                        <td className="p-3">
                          {category.metadata_source && category.metadata_source !== "none" && (
                            <span className="text-xs text-muted-foreground">
                              {t(`categories.metadataSource.options.${category.metadata_source}`)}
                            </span>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Card>
          </div>

          {/* Mobile Card View */}
          <div className="md:hidden grid gap-2">
            {filteredCategories.map((category) => (
              <Card
                key={category.id}
                className="group relative py-0 transition-[filter] duration-200 saturate-100 hover:saturate-[2] overflow-hidden"
              >
                <div
                  className="absolute inset-0 pointer-events-none"
                  style={{ background: `linear-gradient(to left, ${category.color || "#3b82f6"}40, transparent 60%)` }}
                />
                <div
                  className="absolute inset-y-0 left-0 w-14 flex items-center justify-center flex-shrink-0"
                  style={{ backgroundColor: category.color || "#3b82f6" }}
                >
                  {(() => {
                    const IconComponent = getCategoryIcon(category.icon);
                    return <IconComponent className="h-6 w-6 text-white" />;
                  })()}
                </div>
                <div className="absolute inset-0 bg-black/20 dark:bg-white/20 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
                <CardContent className="relative pl-[4.5rem] pr-3 py-3 space-y-2">
                  <div className="flex items-center gap-2.5">
                    <h3 className="font-medium text-sm truncate flex-1 min-w-0">{category.name}</h3>
                    {category.metadata_source && category.metadata_source !== "none" && (
                      <span className="text-xs text-muted-foreground flex-shrink-0">
                        {t(`categories.metadataSource.options.${category.metadata_source}`)}
                      </span>
                    )}
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 shrink-0"
                      onClick={() => startEditCategory(category)}
                      aria-label={t("categories.editCategory", { defaultValue: "Edit {{name}}", name: category.name })}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                  </div>

                  {category.default_directories?.length > 0 && (
                    <div className="flex min-w-0 flex-wrap items-center gap-1">
                      <Folder className="h-3 w-3 text-muted-foreground flex-shrink-0" />
                      {category.default_directories.map((directory) => (
                        <span key={directory} className="max-w-full truncate rounded bg-muted px-1.5 py-0.5 text-xs font-mono">
                          {directory}
                        </span>
                      ))}
                    </div>
                  )}

                  {category.default_tags && category.default_tags.length > 0 && (
                    <div className="flex gap-1 flex-wrap items-center">
                      {category.default_tags.slice(0, 3).map((tag) => (
                        <TagBadge
                          key={tag}
                          tag={tag}
                          size="sm"
                          showIcon={false}
                        />
                      ))}
                      {category.default_tags.length > 3 && (
                        <span className="text-xs text-muted-foreground">
                          +{category.default_tags.length - 3}
                        </span>
                      )}
                    </div>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>


          {/* Delete Confirmation Modal */}
          <Dialog open={showDeleteModal} onOpenChange={setShowDeleteModal}>
            <DialogContent className="max-w-md">
              <DialogHeader>
                <DialogTitle>{t('categories.deleteConfirmTitle')}</DialogTitle>
              </DialogHeader>
              
              {categoryToDelete && (
                <div className="space-y-4">
                  <p className="text-sm text-muted-foreground">
                    {t('categories.deleteConfirmMessage', { name: categoryToDelete.name })}
                  </p>
                  
                  <div className="flex items-center gap-3 p-3 bg-accent/50 rounded-lg">
                    <div
                      className="w-12 h-12 rounded-lg flex items-center justify-center flex-shrink-0"
                      style={{ backgroundColor: categoryToDelete.color || "#3b82f6" }}
                    >
                      {(() => {
                        const IconComponent = getCategoryIcon(categoryToDelete.icon);
                        return <IconComponent className="h-6 w-6 text-white" />;
                      })()}
                    </div>
                    <div className="min-w-0 flex-1">
                      <h3 className="font-semibold text-sm">{categoryToDelete.name}</h3>
                      {categoryToDelete.default_tags && categoryToDelete.default_tags.length > 0 && (
                        <p className="text-xs text-muted-foreground">
                          {categoryToDelete.default_tags.length} {t('categories.fields.defaultTags').toLowerCase()}
                        </p>
                      )}
                    </div>
                  </div>

                  <div className="flex gap-2 justify-end">
                    <Button 
                      variant="outline" 
                      onClick={() => {
                        setShowDeleteModal(false);
                        setCategoryToDelete(null);
                      }}
                    >
                      {t('common.cancel')}
                    </Button>
                    <Button 
                      variant="destructive" 
                      onClick={handleDeleteCategory}
                    >
                      <Trash2 className="h-4 w-4 mr-2" />
                      {t('common.delete')}
                    </Button>
                  </div>
                </div>
              )}
            </DialogContent>
          </Dialog>

        </>
      )}

          {/* Add/Edit Category Modal - Always available */}
          <AddCategoryModal
            open={showCreateModal}
            onOpenChange={handleModalClose}
            onCategoryCreated={handleCreateCategory}
            editingCategory={editingCategory}
            onCategoryUpdated={handleUpdateCategory}
          />
    </div>
  );
}

export default Categories;
