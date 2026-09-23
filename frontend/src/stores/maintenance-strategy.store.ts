import { Injectable, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { MaintenanceStrategy } from '../models';
import { strategyListApi } from '../api/maintenance-strategy.api';

@Injectable({ providedIn: 'root' })
export class MaintenanceStrategyStore {
  readonly list = signal<MaintenanceStrategy[]>([]);
  readonly loading = signal(false);

  constructor(private http: HttpClient) {}

  load(category?: string): void {
    this.loading.set(true);
    strategyListApi(this.http, category).subscribe({
      next: (res: MaintenanceStrategy[]) => {
        this.list.set(res);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }
}
