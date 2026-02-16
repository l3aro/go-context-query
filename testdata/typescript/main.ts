import { HelperClass } from './helper';
import { MathUtils } from './utils/math';

export class MainClass {
    private helper: HelperClass;

    constructor() {
        this.helper = new HelperClass();
    }

    public main(): number {
        // Call local method
        const localResult = this.localMethod();

        // Call method from another class
        const helperResult = this.helper.doSomething();

        // Call static method from utility class
        const mathResult = MathUtils.add(1, 2);

        return localResult + helperResult + mathResult;
    }

    private localMethod(): number {
        return 10;
    }

    public callerMethod(): number {
        const a = this.localMethod();
        const b = this.helper.doSomething();
        const c = MathUtils.multiply(a, b);
        return c;
    }
}
