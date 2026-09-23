import { Component, OnInit, effect, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormsModule, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTableModule } from '@angular/material/table';
import { MatSnackBar, MatSnackBarModule } from '@angular/material/snack-bar';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MaintenanceStrategy } from '../../../models';
import { MaintenanceStrategyStore } from '../../../stores/maintenance-strategy.store';
import { AuthStore } from '../../../stores/auth.store';
import { strategyCreateApi, strategyUpdateApi, CreateStrategyPayload } from '../../../api/maintenance-strategy.api';
import { DEVICE_CATEGORIES, MAINTENANCE_TYPE_TEXT, ROLE } from '../../../constants/enums';
import { parseHttpError, useHttp } from '../../../utils/request';

const PLAN_TYPE_OPTIONS = [
  { value: 'daily', label: '日检' },
  { value: 'weekly', label: '周检' },
  { value: 'monthly', label: '月检' },
  { value: 'yearly', label: '年检' },
];

@Component({
  selector: 'app-strategy-dialog',
  standalone: true,
  imports: [
    CommonModule, FormsModule, ReactiveFormsModule, MatDialogModule, MatFormFieldModule, MatInputModule,
    MatSelectModule, MatButtonModule, MatIconModule, MatTableModule, MatSnackBarModule, MatProgressSpinnerModule,
  ],
  template: `
    <h2 mat-dialog-title>保养周期策略（按设备类别）</h2>
    <mat-dialog-content>
      <p class="hint">生成计划时仅处理启用策略；同类别同类型仅一条策略，可调整周期或停用。</p>
      <form *ngIf="canManage" class="create-bar" [formGroup]="createForm" (ngSubmit)="create()">
        <mat-form-field appearance="outline">
          <mat-label>设备类别</mat-label>
          <mat-select formControlName="category">
            <mat-option *ngFor="let c of categories" [value]="c">{{ c }}</mat-option>
          </mat-select>
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>保养类型</mat-label>
          <mat-select formControlName="type">
            <mat-option *ngFor="let t of typeOptions" [value]="t.value">{{ t.label }}</mat-option>
          </mat-select>
        </mat-form-field>
        <mat-form-field appearance="outline">
          <mat-label>周期(天)</mat-label>
          <input matInput type="number" min="1" formControlName="interval_days">
        </mat-form-field>
        <button mat-flat-button color="primary" type="submit" [disabled]="createForm.invalid || saving">
          <mat-icon>add</mat-icon> 新增策略
        </button>
      </form>
      <table mat-table [dataSource]="store.list()" class="full-table">
        <ng-container matColumnDef="category">
          <th mat-header-cell *matHeaderCellDef>设备类别</th>
          <td mat-cell *matCellDef="let s">{{ s.category }}</td>
        </ng-container>
        <ng-container matColumnDef="type">
          <th mat-header-cell *matHeaderCellDef>保养类型</th>
          <td mat-cell *matCellDef="let s">{{ typeText[s.type] || s.type }}</td>
        </ng-container>
        <ng-container matColumnDef="interval_days">
          <th mat-header-cell *matHeaderCellDef>周期(天)</th>
          <td mat-cell *matCellDef="let s">
            <input class="interval-input" type="number" min="1" [(ngModel)]="edits[s.id]" [disabled]="!canManage">
          </td>
        </ng-container>
        <ng-container matColumnDef="enabled">
          <th mat-header-cell *matHeaderCellDef>状态</th>
          <td mat-cell *matCellDef="let s">
            <span class="tag" [class.tag-off]="!s.enabled">{{ s.enabled ? '启用中' : '已停用' }}</span>
          </td>
        </ng-container>
        <ng-container matColumnDef="actions">
          <th mat-header-cell *matHeaderCellDef>操作</th>
          <td mat-cell *matCellDef="let s">
            <ng-container *ngIf="canManage">
              <button mat-stroked-button color="primary" [disabled]="saving || edits[s.id] === s.interval_days || !edits[s.id]" (click)="saveInterval(s)">保存周期</button>
              <button mat-stroked-button [color]="s.enabled ? 'warn' : 'accent'" [disabled]="saving" (click)="toggle(s)">
                {{ s.enabled ? '停用' : '启用' }}
              </button>
            </ng-container>
            <span *ngIf="!canManage">-</span>
          </td>
        </ng-container>
        <tr mat-header-row *matHeaderRowDef="columns"></tr>
        <tr mat-row *matRowDef="let row; columns: columns;"></tr>
      </table>
      <div class="loading" *ngIf="store.loading()"><mat-spinner diameter="30"></mat-spinner></div>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button mat-button (click)="dialogRef.close()">关闭</button>
    </mat-dialog-actions>
  `,
  styles: [`
    .hint { color: #666; font-size: 13px; margin: 0 0 8px; }
    .create-bar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
    .create-bar mat-form-field { width: 150px; }
    .full-table { width: 100%; }
    .full-table button { margin-right: 4px; }
    .interval-input { width: 72px; padding: 4px 6px; border: 1px solid #ccc; border-radius: 4px; }
    .tag { color: #2e7d32; font-weight: 600; }
    .tag-off { color: #c62828; }
    .loading { display: flex; justify-content: center; padding: 16px; }
  `],
})
export class StrategyDialogComponent implements OnInit {
  private http = useHttp();
  private fb = inject(FormBuilder);
  private snackBar = inject(MatSnackBar);
  store = inject(MaintenanceStrategyStore);
  auth = inject(AuthStore);

  categories = DEVICE_CATEGORIES;
  typeOptions = PLAN_TYPE_OPTIONS;
  typeText = MAINTENANCE_TYPE_TEXT;
  columns = ['category', 'type', 'interval_days', 'enabled', 'actions'];
  edits: Record<number, number> = {};
  saving = false;

  createForm = this.fb.nonNullable.group({
    category: ['', Validators.required],
    type: ['daily', Validators.required],
    interval_days: [1, [Validators.required, Validators.min(1)]],
  });

  get canManage(): boolean {
    return this.auth.hasRole(ROLE.DEVICE_ADMIN, ROLE.SUPER_ADMIN);
  }

  constructor(public dialogRef: MatDialogRef<StrategyDialogComponent>) {
    // 列表刷新后同步行内周期编辑值。
    effect(() => {
      const map: Record<number, number> = {};
      for (const s of this.store.list()) map[s.id] = s.interval_days;
      this.edits = map;
    });
  }

  ngOnInit(): void {
    this.store.load();
  }

  create(): void {
    if (this.createForm.invalid) return;
    const raw = this.createForm.getRawValue();
    const payload: CreateStrategyPayload = { category: raw.category, type: raw.type, interval_days: raw.interval_days };
    this.saving = true;
    strategyCreateApi(this.http, payload).subscribe({
      next: () => {
        this.saving = false;
        this.snackBar.open('策略已创建', '关闭', { duration: 2000 });
        this.store.load();
      },
      error: (err) => {
        this.saving = false;
        this.snackBar.open(parseHttpError(err), '关闭', { duration: 3000 });
      },
    });
  }

  saveInterval(s: MaintenanceStrategy): void {
    const days = this.edits[s.id];
    if (!days || days < 1) return;
    this.saving = true;
    strategyUpdateApi(this.http, s.id, { interval_days: days }).subscribe({
      next: () => {
        this.saving = false;
        this.snackBar.open(`已调整「${s.category}」周期为 ${days} 天`, '关闭', { duration: 2000 });
        this.store.load();
      },
      error: (err) => {
        this.saving = false;
        this.snackBar.open(parseHttpError(err), '关闭', { duration: 3000 });
      },
    });
  }

  toggle(s: MaintenanceStrategy): void {
    this.saving = true;
    strategyUpdateApi(this.http, s.id, { enabled: !s.enabled }).subscribe({
      next: () => {
        this.saving = false;
        this.snackBar.open(s.enabled ? '策略已停用' : '策略已启用', '关闭', { duration: 2000 });
        this.store.load();
      },
      error: (err) => {
        this.saving = false;
        this.snackBar.open(parseHttpError(err), '关闭', { duration: 3000 });
      },
    });
  }
}
