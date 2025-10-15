import { writable } from 'svelte/store';
import { apiJSON } from '$lib/api';
import { t } from '$lib/i18n';

export interface ClassSummary {
  id: string | number;
  name: string;
  teacher_id?: string | number;
  description?: string;
  created_at?: string;
  updated_at?: string;
}

interface ClassesStore {
  classes: ClassSummary[];
  loading: boolean;
  error: string | null;
}

function createClassesStore() {
  const { subscribe, set, update } = writable<ClassesStore>({
    classes: [],
    loading: false,
    error: null
  });

  return {
    subscribe,
    
    // Load classes for the current user
    async load() {
      update(state => ({ ...state, loading: true, error: null }));
      try {
        const result = await apiJSON('/api/classes');
        const classes = Array.isArray(result) ? result : [];
        set({ classes, loading: false, error: null });
        return classes;
      } catch (error: any) {
        const errorMessage = error?.message ?? t('frontend/src/lib/stores/classes.ts::failed-to-load-classes');
        set({ classes: [], loading: false, error: errorMessage });
        throw error;
      }
    },

    // Add a new class to the store
    addClass(newClass: ClassSummary) {
      update(state => ({
        ...state,
        classes: [newClass, ...state.classes] // Add to beginning since classes are ordered by created_at DESC
      }));
    },

    // Update an existing class
    updateClass(id: string | number, updates: Partial<ClassSummary>) {
      update(state => ({
        ...state,
        classes: state.classes.map(c => 
          c.id === id ? { ...c, ...updates } : c
        )
      }));
    },

    // Remove a class from the store
    removeClass(id: string | number) {
      update(state => ({
        ...state,
        classes: state.classes.filter(c => c.id !== id)
      }));
    },

    // Clear the store
    clear() {
      set({ classes: [], loading: false, error: null });
    },

    // Set classes directly (useful for admin panel)
    setClasses(classes: ClassSummary[]) {
      set({ classes, loading: false, error: null });
    }
  };
}

export const classesStore = createClassesStore();
