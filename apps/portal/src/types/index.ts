export type App = {
  id: string;
  name: string;
  description: string;
  icon: string;
  href: string;
};

export type Category = {
  id: string;
  title: string;
  apps: App[];
};
