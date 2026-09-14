import { createContext, type Dispatch, type SetStateAction } from "react";

export interface SidebarContextType {
  state: "expanded" | "collapsed";
  open: boolean;
  setOpen: (open: boolean | ((current: boolean) => boolean)) => void;
  sidebarWidth: number;
  setSidebarWidth: (width: number | ((current: number) => number)) => void;
  minSidebarWidth: number;
  maxSidebarWidth: number;
  resizable: boolean;
  openMobile: boolean;
  setOpenMobile: Dispatch<SetStateAction<boolean>>;
  isMobile: boolean;
  toggleSidebar: () => void;
}

export const SidebarContext = createContext<SidebarContextType | undefined>(undefined);
